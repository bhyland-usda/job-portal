package user

import (
	"context"
	"database/sql"
)

type ProfileCompleteness struct {
	Score   int
	Total   int
	Percent int
	Items   []CompletenessItem
}

type CompletenessItem struct {
	Label    string
	Complete bool
}

func GetProfileCompleteness(ctx context.Context, db *sql.DB, userID string) ProfileCompleteness {
	items := []CompletenessItem{
		{Label: "Add a headline", Complete: false},
		{Label: "Add a location", Complete: false},
		{Label: "Write an about section", Complete: false},
		{Label: "Upload a photo", Complete: false},
		{Label: "Add experience", Complete: false},
		{Label: "Add education", Complete: false},
		{Label: "Add skills", Complete: false},
	}

	var headline, location, about, avatarURL sql.NullString
	db.QueryRowContext(ctx,
		`SELECT headline, location, about, avatar_url FROM users WHERE id = $1`, userID,
	).Scan(&headline, &location, &about, &avatarURL)

	if headline.Valid && headline.String != "" {
		items[0].Complete = true
	}
	if location.Valid && location.String != "" {
		items[1].Complete = true
	}
	if about.Valid && about.String != "" {
		items[2].Complete = true
	}
	if avatarURL.Valid && avatarURL.String != "" {
		items[3].Complete = true
	}

	var expCount, eduCount, skillCount int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experiences WHERE user_id = $1`, userID).Scan(&expCount)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM educations WHERE user_id = $1`, userID).Scan(&eduCount)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skills WHERE user_id = $1`, userID).Scan(&skillCount)

	if expCount > 0 {
		items[4].Complete = true
	}
	if eduCount > 0 {
		items[5].Complete = true
	}
	if skillCount > 0 {
		items[6].Complete = true
	}

	score := 0
	for _, item := range items {
		if item.Complete {
			score++
		}
	}

	pct := 0
	if len(items) > 0 {
		pct = (score * 100) / len(items)
	}

	return ProfileCompleteness{
		Score:   score,
		Total:   len(items),
		Percent: pct,
		Items:   items,
	}
}
