package db

import (
	"fmt"
	"sort"
	"time"
)

// DayCount is the event count for one calendar day (UTC).
type DayCount struct {
	Day   string `json:"day"` // YYYY-MM-DD
	Count int64  `json:"count"`
}

// EventReport aggregates motion events over a period: total, per day (ordered),
// per raw label and per camera. A categoria (movimento/pessoa/ia) é derivada do
// label no frontend (mesma regra do eventCategory), por isso aqui vai por label.
type EventReport struct {
	Total    int64            `json:"total"`
	ByDay    []DayCount       `json:"by_day"`
	ByLabel  map[string]int64 `json:"by_label"`
	ByCamera map[string]int64 `json:"by_camera"`
}

// AggregateMotionEvents conta os motion_events em [from, to) por dia, por label
// (cru) e por câmera. occurred_at é RFC3339 (string lexicograficamente ordenável).
func AggregateMotionEvents(db *DB, from, to time.Time) (EventReport, error) {
	rows, err := db.Query(
		`SELECT occurred_at, camera_id, COALESCE(label, '') FROM motion_events
		 WHERE occurred_at >= ? AND occurred_at < ?`,
		from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return EventReport{}, fmt.Errorf("aggregate motion events: %w", err)
	}
	defer rows.Close()

	day := map[string]int64{}
	byLabel := map[string]int64{}
	byCamera := map[string]int64{}
	var total int64
	for rows.Next() {
		var occurredAt, camID, label string
		if err := rows.Scan(&occurredAt, &camID, &label); err != nil {
			return EventReport{}, fmt.Errorf("scan event row: %w", err)
		}
		t, _ := time.Parse(time.RFC3339, occurredAt)
		total++
		day[t.UTC().Format("2006-01-02")]++
		byCamera[camID]++
		byLabel[label]++
	}
	if err := rows.Err(); err != nil {
		return EventReport{}, err
	}

	days := make([]string, 0, len(day))
	for d := range day {
		days = append(days, d)
	}
	sort.Strings(days)
	byDay := make([]DayCount, 0, len(days))
	for _, d := range days {
		byDay = append(byDay, DayCount{Day: d, Count: day[d]})
	}
	return EventReport{Total: total, ByDay: byDay, ByLabel: byLabel, ByCamera: byCamera}, nil
}
