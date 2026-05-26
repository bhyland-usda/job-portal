package onboarding

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Step struct {
	ID          string
	Title       string
	Description string
	SortOrder   int
	CompletedAt *time.Time
}

type OnboardingPage struct {
	middleware.BaseData
	Steps     []Step
	Completed int
	Total     int
	Percent   int
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /onboarding", requireAuth(http.HandlerFunc(h.showOnboarding)))
	mux.Handle("POST /onboarding/{id}/complete", requireAuth(http.HandlerFunc(h.completeStep)))
}

func (h *Handler) showOnboarding(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT os.id, os.title, os.description, os.sort_order, uo.completed_at
		 FROM onboarding_steps os
		 LEFT JOIN user_onboarding uo ON uo.step_id = os.id AND uo.user_id = $1
		 ORDER BY os.sort_order`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load onboarding", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var steps []Step
	completed := 0
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.SortOrder, &s.CompletedAt); err != nil {
			continue
		}
		if s.CompletedAt != nil {
			completed++
		}
		steps = append(steps, s)
	}

	pct := 0
	if len(steps) > 0 {
		pct = (completed * 100) / len(steps)
	}

	data := OnboardingPage{
		BaseData:  middleware.NewBaseData(r),
		Steps:     steps,
		Completed: completed,
		Total:     len(steps),
		Percent:   pct,
	}
	h.pages["onboarding.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) completeStep(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	stepID := r.PathValue("id")

	h.db.ExecContext(r.Context(),
		`INSERT INTO user_onboarding (user_id, step_id, completed_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id, step_id) DO NOTHING`,
		userID, stepID,
	)

	http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
}
