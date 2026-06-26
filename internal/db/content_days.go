package db

import (
	"fmt"
	"sort"
	"time"
)

// ContentDays devolve as datas locais distintas (YYYY-MM-DD, ordenadas) em que a
// câmera tem gravação OU evento de movimento — a união das duas fontes. Usado
// pelos calendários para habilitar só os dias com conteúdo.
//
// Os timestamps são gravados em UTC (RFC3339); o SQLite converte para o dia
// local aplicando o offset de loc via date(ts, '±HH:MM'). O offset é fixo
// (calculado em "agora"), assumindo sem horário de verão — o Brasil não tem DST
// desde 2019 e este é um sistema residencial. Dias de transição de DST (outros
// fusos) poderiam cair no dia vizinho; aceitável para o caso de uso.
func ContentDays(database *DB, cameraID string, loc *time.Location) ([]string, error) {
	if loc == nil {
		loc = time.UTC
	}
	offset := sqliteOffset(loc)

	set := make(map[string]struct{})
	queries := []string{
		`SELECT DISTINCT date(started_at, ?) FROM recordings WHERE camera_id=?`,
		`SELECT DISTINCT date(occurred_at, ?) FROM motion_events WHERE camera_id=?`,
	}
	for _, q := range queries {
		rows, err := database.Query(q, offset, cameraID)
		if err != nil {
			return nil, fmt.Errorf("content days: %w", err)
		}
		for rows.Next() {
			var day string
			if err := rows.Scan(&day); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan content day: %w", err)
			}
			if day != "" {
				set[day] = struct{}{}
			}
		}
		rows.Close()
	}

	days := make([]string, 0, len(set))
	for day := range set {
		days = append(days, day)
	}
	sort.Strings(days)
	return days, nil
}

// sqliteOffset formata o offset de loc (em "agora") como o modifier do SQLite
// date(): "+HH:MM" / "-HH:MM".
func sqliteOffset(loc *time.Location) string {
	_, secs := time.Now().In(loc).Zone()
	sign := "+"
	if secs < 0 {
		sign = "-"
		secs = -secs
	}
	return fmt.Sprintf("%s%02d:%02d", sign, secs/3600, (secs%3600)/60)
}
