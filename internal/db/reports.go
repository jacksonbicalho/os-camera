package db

import (
	"fmt"
	"regexp"
	"time"
)

// DayCount is the event count for one calendar day (UTC), com a quebra por categoria
// (movimento/pessoa/ia/estados) para as barras empilhadas.
type DayCount struct {
	Day        string           `json:"day"` // YYYY-MM-DD
	Count      int64            `json:"count"`
	ByCategory map[string]int64 `json:"by_category"`
}

var personRe = regexp.MustCompile(`(?i)pessoa|person`)

// MotionCategory deriva a categoria de um motion event pelo label — mesma regra do
// eventCategory no frontend: vazio→movimento, pessoa/person→pessoa, resto→ia.
func MotionCategory(label string) string {
	if label == "" {
		return "movimento"
	}
	if personRe.MatchString(label) {
		return "pessoa"
	}
	return "ia"
}

// HourCount is the event count for one hour-of-day (0..23) no fuso pedido, com a quebra
// por categoria — usado no modo "dia" (barras por hora).
type HourCount struct {
	Hour       int              `json:"hour"`
	Count      int64            `json:"count"`
	ByCategory map[string]int64 `json:"by_category"`
}

// HeatCell is the event count for one (weekday, hour-of-day) bucket no fuso pedido —
// weekday 0=domingo..6=sábado (time.Weekday), hour 0..23. Usado no heatmap temporal
// (mapa de atividade dia-da-semana × hora) acumulado sobre vários dias.
type HeatCell struct {
	Weekday int   `json:"weekday"`
	Hour    int   `json:"hour"`
	Count   int64 `json:"count"`
}

// EventReport aggregates events of a single camera over a period: total, per day
// (ordered), per raw motion label and per category. As categorias movimento/pessoa/ia
// são derivadas do label no frontend (mesma regra do eventCategory); `estados` (que não
// vem de label) é contada aqui em ByCategory a partir de camera_state_history.
// ByHour só é preenchido no modo por hora (AggregateMotionEventsHourly); ByDay no diário.
type EventReport struct {
	Total      int64            `json:"total"`
	ByDay      []DayCount       `json:"by_day"`
	ByHour     []HourCount      `json:"by_hour,omitempty"`
	ByLabel    map[string]int64 `json:"by_label"`
	ByCategory map[string]int64 `json:"by_category"`
	Heatmap    []HeatCell       `json:"heatmap,omitempty"`
}

// AggregateMotionEvents conta os eventos de UMA câmera em [from, to): motion_events
// (por dia e por label cru) somados às transições de estado (camera_state_history,
// contabilizadas na categoria "estados"). occurred_at é RFC3339; changed_at é
// 'YYYY-MM-DD HH:MM:SS', por isso a comparação de tempo do histórico passa por datetime().
func AggregateMotionEvents(db *DB, from, to time.Time, cameraID string) (EventReport, error) {
	dayCat := map[string]map[string]int64{}
	addDay := func(d, cat string) {
		if dayCat[d] == nil {
			dayCat[d] = map[string]int64{}
		}
		dayCat[d][cat]++
	}
	byLabel := map[string]int64{}
	byCategory := map[string]int64{}
	var total int64

	rows, err := db.Query(
		`SELECT occurred_at, COALESCE(label, '') FROM motion_events
		 WHERE camera_id = ? AND occurred_at >= ? AND occurred_at < ?`,
		cameraID, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate motion events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var occurredAt, label string
		if err := rows.Scan(&occurredAt, &label); err != nil {
			return EventReport{}, fmt.Errorf("scan event row: %w", err)
		}
		t, _ := time.Parse(time.RFC3339, occurredAt)
		total++
		addDay(t.UTC().Format("2006-01-02"), MotionCategory(label))
		byLabel[label]++
	}
	if err := rows.Err(); err != nil {
		return EventReport{}, err
	}

	const tsLayout = "2006-01-02 15:04:05"
	hRows, err := db.Query(
		`SELECT h.changed_at
		 FROM camera_state_history h
		 JOIN camera_state_classifiers c ON c.id = h.classifier_id
		 WHERE c.camera_id = ?
		   AND datetime(h.changed_at) >= datetime(?)
		   AND datetime(h.changed_at) < datetime(?)`,
		cameraID, from.UTC().Format(tsLayout), to.UTC().Format(tsLayout),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate state history: %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var changedAt time.Time
		if err := hRows.Scan(&changedAt); err != nil {
			return EventReport{}, fmt.Errorf("scan state row: %w", err)
		}
		total++
		addDay(changedAt.UTC().Format("2006-01-02"), "estados")
		byCategory["estados"]++
	}
	if err := hRows.Err(); err != nil {
		return EventReport{}, err
	}

	// Preenche TODOS os dias UTC da janela [from, to) — inclusive os sem evento — para
	// o gráfico virar uma linha do tempo contínua (dias vazios = barra zero).
	byDay := []DayCount{}
	startDay := time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	for d := startDay; d.Before(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		counts := dayCat[key]
		if counts == nil {
			counts = map[string]int64{}
		}
		var c int64
		for _, n := range counts {
			c += n
		}
		byDay = append(byDay, DayCount{Day: key, Count: c, ByCategory: counts})
	}
	return EventReport{Total: total, ByDay: byDay, ByLabel: byLabel, ByCategory: byCategory}, nil
}

// AggregateMotionEventsHourly conta os eventos de UMA câmera em [from, to) — tipicamente
// um único dia — agrupando por hora-do-dia (0..23) no fuso `loc`, com a mesma quebra por
// categoria do diário (motion via label + estados de camera_state_history). Devolve 24
// buckets (zero-fill) em ByHour; ByDay fica nil.
func AggregateMotionEventsHourly(db *DB, from, to time.Time, cameraID string, loc *time.Location) (EventReport, error) {
	if loc == nil {
		loc = time.UTC
	}
	hourCat := map[int]map[string]int64{}
	addHour := func(h int, cat string) {
		if hourCat[h] == nil {
			hourCat[h] = map[string]int64{}
		}
		hourCat[h][cat]++
	}
	byLabel := map[string]int64{}
	byCategory := map[string]int64{}
	var total int64

	rows, err := db.Query(
		`SELECT occurred_at, COALESCE(label, '') FROM motion_events
		 WHERE camera_id = ? AND occurred_at >= ? AND occurred_at < ?`,
		cameraID, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate motion events (hourly): %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var occurredAt, label string
		if err := rows.Scan(&occurredAt, &label); err != nil {
			return EventReport{}, fmt.Errorf("scan event row: %w", err)
		}
		t, _ := time.Parse(time.RFC3339, occurredAt)
		total++
		addHour(t.In(loc).Hour(), MotionCategory(label))
		byLabel[label]++
	}
	if err := rows.Err(); err != nil {
		return EventReport{}, err
	}

	const tsLayout = "2006-01-02 15:04:05"
	hRows, err := db.Query(
		`SELECT h.changed_at
		 FROM camera_state_history h
		 JOIN camera_state_classifiers c ON c.id = h.classifier_id
		 WHERE c.camera_id = ?
		   AND datetime(h.changed_at) >= datetime(?)
		   AND datetime(h.changed_at) < datetime(?)`,
		cameraID, from.UTC().Format(tsLayout), to.UTC().Format(tsLayout),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate state history (hourly): %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var changedAt time.Time
		if err := hRows.Scan(&changedAt); err != nil {
			return EventReport{}, fmt.Errorf("scan state row: %w", err)
		}
		total++
		addHour(changedAt.In(loc).Hour(), "estados")
		byCategory["estados"]++
	}
	if err := hRows.Err(); err != nil {
		return EventReport{}, err
	}

	byHour := make([]HourCount, 24)
	for h := 0; h < 24; h++ {
		counts := hourCat[h]
		if counts == nil {
			counts = map[string]int64{}
		}
		var c int64
		for _, n := range counts {
			c += n
		}
		byHour[h] = HourCount{Hour: h, Count: c, ByCategory: counts}
	}
	return EventReport{Total: total, ByHour: byHour, ByLabel: byLabel, ByCategory: byCategory}, nil
}

// AggregateMotionEventsHeatmap conta os eventos de UMA câmera em [from, to) agrupando por
// (dia-da-semana, hora-do-dia) no fuso `loc` — mesmo conjunto de fontes dos demais
// agregadores (motion_events + camera_state_history). Devolve as 168 células (7×24)
// zero-fill, ordenadas por weekday e depois hour. Total = soma das células.
func AggregateMotionEventsHeatmap(db *DB, from, to time.Time, cameraID string, loc *time.Location) (EventReport, error) {
	if loc == nil {
		loc = time.UTC
	}
	cell := map[[2]int]int64{}
	var total int64

	rows, err := db.Query(
		`SELECT occurred_at FROM motion_events
		 WHERE camera_id = ? AND occurred_at >= ? AND occurred_at < ?`,
		cameraID, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate motion events (heatmap): %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var occurredAt string
		if err := rows.Scan(&occurredAt); err != nil {
			return EventReport{}, fmt.Errorf("scan event row: %w", err)
		}
		t, _ := time.Parse(time.RFC3339, occurredAt)
		lt := t.In(loc)
		cell[[2]int{int(lt.Weekday()), lt.Hour()}]++
		total++
	}
	if err := rows.Err(); err != nil {
		return EventReport{}, err
	}

	const tsLayout = "2006-01-02 15:04:05"
	hRows, err := db.Query(
		`SELECT h.changed_at
		 FROM camera_state_history h
		 JOIN camera_state_classifiers c ON c.id = h.classifier_id
		 WHERE c.camera_id = ?
		   AND datetime(h.changed_at) >= datetime(?)
		   AND datetime(h.changed_at) < datetime(?)`,
		cameraID, from.UTC().Format(tsLayout), to.UTC().Format(tsLayout),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate state history (heatmap): %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var changedAt time.Time
		if err := hRows.Scan(&changedAt); err != nil {
			return EventReport{}, fmt.Errorf("scan state row: %w", err)
		}
		lt := changedAt.In(loc)
		cell[[2]int{int(lt.Weekday()), lt.Hour()}]++
		total++
	}
	if err := hRows.Err(); err != nil {
		return EventReport{}, err
	}

	heatmap := make([]HeatCell, 0, 168)
	for wd := 0; wd < 7; wd++ {
		for h := 0; h < 24; h++ {
			heatmap = append(heatmap, HeatCell{Weekday: wd, Hour: h, Count: cell[[2]int{wd, h}]})
		}
	}
	return EventReport{Total: total, Heatmap: heatmap}, nil
}
