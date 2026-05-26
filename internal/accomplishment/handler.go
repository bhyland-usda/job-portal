package accomplishment

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Accomplishment struct {
	ID          string
	UserID      string
	Title       string
	Description string
	PeriodType  string
	PeriodStart time.Time
	PeriodEnd   time.Time
	CreatedAt   time.Time
}

type ListPage struct {
	middleware.BaseData
	Accomplishments []Accomplishment
	PeriodFilter    string
}

type FormPage struct {
	middleware.BaseData
	Accomplishment *Accomplishment
	Error          string
}

type ExportPage struct {
	middleware.BaseData
	Years []YearGroup
}

type YearGroup struct {
	Year      int
	Quarterly []Accomplishment
	Yearly    []Accomplishment
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /accomplishments", requireAuth(http.HandlerFunc(h.showList)))
	mux.Handle("GET /accomplishments/add", requireAuth(http.HandlerFunc(h.showAdd)))
	mux.Handle("POST /accomplishments/add", requireAuth(http.HandlerFunc(h.handleAdd)))
	mux.Handle("GET /accomplishments/{id}/edit", requireAuth(http.HandlerFunc(h.showEdit)))
	mux.Handle("POST /accomplishments/{id}/edit", requireAuth(http.HandlerFunc(h.handleEdit)))
	mux.Handle("POST /accomplishments/{id}/delete", requireAuth(http.HandlerFunc(h.handleDelete)))
	mux.Handle("GET /accomplishments/export", requireAuth(http.HandlerFunc(h.handleExport)))
}

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	periodFilter := r.URL.Query().Get("period")

	query := `SELECT id, user_id, title, description, period_type, period_start, period_end, created_at
		FROM accomplishments
		WHERE user_id = $1`
	args := []interface{}{userID}

	if periodFilter == "quarterly" || periodFilter == "yearly" {
		query += ` AND period_type = $2`
		args = append(args, periodFilter)
	}

	query += ` ORDER BY period_start DESC`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		slog.Error("failed to load accomplishments", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var accomplishments []Accomplishment
	for rows.Next() {
		var a Accomplishment
		if err := rows.Scan(&a.ID, &a.UserID, &a.Title, &a.Description,
			&a.PeriodType, &a.PeriodStart, &a.PeriodEnd, &a.CreatedAt); err != nil {
			slog.Error("failed to scan accomplishment", "error", err)
			continue
		}
		accomplishments = append(accomplishments, a)
	}
	if err := rows.Err(); err != nil {
		slog.Error("rows iteration error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := ListPage{
		BaseData:        middleware.NewBaseData(r),
		Accomplishments: accomplishments,
		PeriodFilter:    periodFilter,
	}

	h.pages["accomplishments.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) showAdd(w http.ResponseWriter, r *http.Request) {
	data := FormPage{BaseData: middleware.NewBaseData(r)}
	h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	periodType := r.FormValue("period_type")
	periodStartStr := r.FormValue("period_start")
	periodEndStr := r.FormValue("period_end")

	if title == "" || description == "" || periodType == "" || periodStartStr == "" || periodEndStr == "" {
		data := FormPage{BaseData: middleware.NewBaseData(r), Error: "All fields are required."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	if periodType != "quarterly" && periodType != "yearly" {
		data := FormPage{BaseData: middleware.NewBaseData(r), Error: "Invalid period type."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	periodStart, err := time.Parse("2006-01-02", periodStartStr)
	if err != nil {
		data := FormPage{BaseData: middleware.NewBaseData(r), Error: "Invalid start date."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	periodEnd, err := time.Parse("2006-01-02", periodEndStr)
	if err != nil {
		data := FormPage{BaseData: middleware.NewBaseData(r), Error: "Invalid end date."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO accomplishments (user_id, title, description, period_type, period_start, period_end)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, title, description, periodType, periodStart, periodEnd,
	)
	if err != nil {
		slog.Error("failed to create accomplishment", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/accomplishments", http.StatusSeeOther)
}

func (h *Handler) showEdit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := r.PathValue("id")

	a, err := h.getOwned(r, id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := FormPage{BaseData: middleware.NewBaseData(r), Accomplishment: a}
	h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := r.PathValue("id")

	existing, err := h.getOwned(r, id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	periodType := r.FormValue("period_type")
	periodStartStr := r.FormValue("period_start")
	periodEndStr := r.FormValue("period_end")

	if title == "" || description == "" || periodType == "" || periodStartStr == "" || periodEndStr == "" {
		data := FormPage{BaseData: middleware.NewBaseData(r), Accomplishment: existing, Error: "All fields are required."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	if periodType != "quarterly" && periodType != "yearly" {
		data := FormPage{BaseData: middleware.NewBaseData(r), Accomplishment: existing, Error: "Invalid period type."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	periodStart, err := time.Parse("2006-01-02", periodStartStr)
	if err != nil {
		data := FormPage{BaseData: middleware.NewBaseData(r), Accomplishment: existing, Error: "Invalid start date."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	periodEnd, err := time.Parse("2006-01-02", periodEndStr)
	if err != nil {
		data := FormPage{BaseData: middleware.NewBaseData(r), Accomplishment: existing, Error: "Invalid end date."}
		h.pages["accomplishment_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE accomplishments
		 SET title = $1, description = $2, period_type = $3, period_start = $4, period_end = $5, updated_at = NOW()
		 WHERE id = $6 AND user_id = $7`,
		title, description, periodType, periodStart, periodEnd, id, userID,
	)
	if err != nil {
		slog.Error("failed to update accomplishment", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/accomplishments", http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM accomplishments WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		slog.Error("failed to delete accomplishment", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/accomplishments", http.StatusSeeOther)
}

func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, user_id, title, description, period_type, period_start, period_end, created_at
		 FROM accomplishments
		 WHERE user_id = $1
		 ORDER BY period_start DESC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load accomplishments for export", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var all []Accomplishment
	for rows.Next() {
		var a Accomplishment
		if err := rows.Scan(&a.ID, &a.UserID, &a.Title, &a.Description,
			&a.PeriodType, &a.PeriodStart, &a.PeriodEnd, &a.CreatedAt); err != nil {
			slog.Error("failed to scan accomplishment for export", "error", err)
			continue
		}
		all = append(all, a)
	}
	if err := rows.Err(); err != nil {
		slog.Error("rows iteration error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Group by year, then by period_type
	yearMap := make(map[int]*YearGroup)
	var yearOrder []int
	for _, a := range all {
		y := a.PeriodStart.Year()
		if _, ok := yearMap[y]; !ok {
			yearMap[y] = &YearGroup{Year: y}
			yearOrder = append(yearOrder, y)
		}
		g := yearMap[y]
		if a.PeriodType == "quarterly" {
			g.Quarterly = append(g.Quarterly, a)
		} else {
			g.Yearly = append(g.Yearly, a)
		}
	}

	// Sort years descending (already mostly desc from query, but ensure order)
	for i := 0; i < len(yearOrder); i++ {
		for j := i + 1; j < len(yearOrder); j++ {
			if yearOrder[j] > yearOrder[i] {
				yearOrder[i], yearOrder[j] = yearOrder[j], yearOrder[i]
			}
		}
	}

	var years []YearGroup
	for _, y := range yearOrder {
		years = append(years, *yearMap[y])
	}

	data := ExportPage{
		BaseData: middleware.NewBaseData(r),
		Years:    years,
	}

	h.pages["accomplishment_export.html"].ExecuteTemplate(w, "accomplishment_base", data)
}

func (h *Handler) getOwned(r *http.Request, id, userID string) (*Accomplishment, error) {
	var a Accomplishment
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, title, description, period_type, period_start, period_end, created_at
		 FROM accomplishments
		 WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&a.ID, &a.UserID, &a.Title, &a.Description,
		&a.PeriodType, &a.PeriodStart, &a.PeriodEnd, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get accomplishment: %w", err)
	}
	return &a, nil
}
