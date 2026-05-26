package badge

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Badge struct {
	ID          string
	Name        string
	Description string
	Icon        string
}

type UserBadge struct {
	Badge    Badge
	EarnedAt time.Time
}

type BadgePage struct {
	middleware.BaseData
	Earned    []UserBadge
	Available []Badge
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /badges", requireAuth(http.HandlerFunc(h.showBadges)))
}

func (h *Handler) showBadges(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Get earned badges
	earnedRows, err := h.db.QueryContext(r.Context(),
		`SELECT b.id, b.name, b.description, b.icon, ub.earned_at
		 FROM user_badges ub
		 JOIN badges b ON b.id = ub.badge_id
		 WHERE ub.user_id = $1
		 ORDER BY ub.earned_at DESC`, userID)
	if err != nil {
		slog.Error("failed to load earned badges", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer earnedRows.Close()

	earnedIDs := make(map[string]bool)
	var earned []UserBadge
	for earnedRows.Next() {
		var ub UserBadge
		if err := earnedRows.Scan(&ub.Badge.ID, &ub.Badge.Name, &ub.Badge.Description,
			&ub.Badge.Icon, &ub.EarnedAt); err != nil {
			slog.Error("failed to scan earned badge", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		earnedIDs[ub.Badge.ID] = true
		earned = append(earned, ub)
	}
	if err := earnedRows.Err(); err != nil {
		slog.Error("failed to iterate earned badges", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get all badges
	allRows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, description, icon FROM badges ORDER BY name`)
	if err != nil {
		slog.Error("failed to load all badges", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer allRows.Close()

	var available []Badge
	for allRows.Next() {
		var b Badge
		if err := allRows.Scan(&b.ID, &b.Name, &b.Description, &b.Icon); err != nil {
			slog.Error("failed to scan badge", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !earnedIDs[b.ID] {
			available = append(available, b)
		}
	}
	if err := allRows.Err(); err != nil {
		slog.Error("failed to iterate badges", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := BadgePage{
		BaseData:  middleware.NewBaseData(r),
		Earned:    earned,
		Available: available,
	}

	if err := h.pages["badges.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render badges", "error", err)
	}
}

// CheckAndAward checks badge conditions and awards any newly earned badges.
// Call this from other handlers after relevant actions.
func CheckAndAward(ctx context.Context, db *sql.DB, userID string) {
	type badgeCheck struct {
		name  string
		query string
		args  []any
	}

	checks := []badgeCheck{
		{
			name:  "First Post",
			query: `SELECT EXISTS(SELECT 1 FROM posts WHERE user_id = $1)`,
			args:  []any{userID},
		},
		{
			name:  "Connector",
			query: `SELECT COUNT(*) >= 10 FROM connections WHERE (requester_id = $1 OR addressee_id = $1) AND status = 'accepted'`,
			args:  []any{userID},
		},
		{
			name: "Profile Complete",
			query: `SELECT
				u.headline IS NOT NULL AND u.headline != '' AND
				u.location IS NOT NULL AND u.location != '' AND
				u.about IS NOT NULL AND u.about != '' AND
				EXISTS(SELECT 1 FROM experiences WHERE user_id = $1) AND
				EXISTS(SELECT 1 FROM educations WHERE user_id = $1) AND
				EXISTS(SELECT 1 FROM skills WHERE user_id = $1)
			FROM users u WHERE u.id = $1`,
			args: []any{userID},
		},
		{
			name:  "Team Player",
			query: `SELECT COUNT(*) >= 5 FROM kudos WHERE receiver_id = $1`,
			args:  []any{userID},
		},
		{
			name:  "Detail Completed",
			query: `SELECT EXISTS(SELECT 1 FROM posting_applications WHERE applicant_id = $1 AND status = 'accepted')`,
			args:  []any{userID},
		},
		{
			name:  "Mentor",
			query: `SELECT EXISTS(SELECT 1 FROM mentorships WHERE mentor_id = $1 AND status IN ('active', 'completed'))`,
			args:  []any{userID},
		},
	}

	for _, c := range checks {
		var met bool
		err := db.QueryRowContext(ctx, c.query, c.args...).Scan(&met)
		if err != nil {
			// Table might not exist yet (e.g., kudos), skip silently
			continue
		}
		if !met {
			continue
		}

		// Award badge if not already earned
		_, err = db.ExecContext(ctx,
			`INSERT INTO user_badges (user_id, badge_id)
			 SELECT $1, id FROM badges WHERE name = $2
			 ON CONFLICT (user_id, badge_id) DO NOTHING`,
			userID, c.name)
		if err != nil {
			slog.Error("failed to award badge", "badge", c.name, "user", userID, "error", err)
		}
	}
}
