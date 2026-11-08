package sources

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"tsuruyomi/internal/database"
)

// PredictNextChapter calculates the release schedule and days until the next expected chapter
// based on historical upload dates for a manga.
func PredictNextChapter(ctx context.Context, mangaID int64) (string, int) {
	if database.DB == nil || mangaID <= 0 {
		return "", -1
	}

	rows, err := database.DB.Query(ctx, `
		SELECT date_upload 
		FROM suwayomi.chapter 
		WHERE manga = $1 AND date_upload > 0 
		ORDER BY source_order DESC 
		LIMIT 15
	`, mangaID)
	if err != nil {
		return "", -1
	}
	defer rows.Close()

	var stamps []int64
	for rows.Next() {
		var s int64
		if err := rows.Scan(&s); err == nil && s > 0 {
			stamps = append(stamps, s)
		}
	}

	if len(stamps) < 3 {
		return "", -1
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	latestPlausible := today.AddDate(0, 0, 1)

	// Collect unique calendar days (UTC midnights)
	seen := make(map[int64]bool)
	var days []time.Time
	for _, stamp := range stamps {
		t := time.UnixMilli(stamp).UTC()
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		if day.After(latestPlausible) {
			continue
		}
		dayKey := day.Unix()
		if !seen[dayKey] {
			seen[dayKey] = true
			days = append(days, day)
		}
	}

	if len(days) < 3 {
		return "", -1
	}

	// Sort descending (most recent first)
	sort.Slice(days, func(i, j int) bool {
		return days[i].After(days[j])
	})

	// Take up to 10 recent release days
	limit := 10
	if len(days) <= 8 {
		limit = 3
	}
	if len(days) < limit {
		limit = len(days)
	}
	history := days[:limit]

	// Compute gaps
	var gaps []int
	for i := 0; i < len(history)-1; i++ {
		gap := int(math.Round(history[i].Sub(history[i+1]).Hours() / 24))
		if gap > 0 {
			gaps = append(gaps, gap)
		}
	}

	if len(gaps) == 0 {
		return "", -1
	}

	sort.Ints(gaps)
	medianGap := gaps[(len(gaps)-1)/2]
	if medianGap < 1 {
		medianGap = 1
	}
	if medianGap > 28 {
		medianGap = 28
	}

	anchor := history[0]
	due := anchor.AddDate(0, 0, medianGap)
	for due.Before(today) {
		due = due.AddDate(0, 0, medianGap)
	}

	daysUntil := int(math.Round(due.Sub(today).Hours() / 24))
	if daysUntil < 0 {
		daysUntil = 0
	}

	var text string
	switch daysUntil {
	case 0:
		text = "Esperado hoy"
	case 1:
		text = "Esperado mañana (en 1 día)"
	default:
		text = fmt.Sprintf("Esperado en %d días", daysUntil)
	}

	return text, daysUntil
}
