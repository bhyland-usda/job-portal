package foia

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Export struct {
	ID          string
	RequestedBy string
	QueryParams string
	Status      string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type FOIAPage struct {
	middleware.BaseData
	Exports []Export
}

type SearchPage struct {
	middleware.BaseData
	UserName string
	DateFrom string
	DateTo   string
	Type     string
	Results  []SearchResult
	Searched bool
}

type SearchResult struct {
	Type      string
	Content   string
	AuthorID  string
	Author    string
	CreatedAt time.Time
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/foia", requireAuth(requireAdmin(http.HandlerFunc(h.showFOIA))))
	mux.Handle("GET /admin/foia/search", requireAuth(requireAdmin(http.HandlerFunc(h.searchData))))
	mux.Handle("GET /admin/foia/export", requireAuth(requireAdmin(http.HandlerFunc(h.exportData))))
}

func (h *Handler) showFOIA(w http.ResponseWriter, r *http.Request) {
	data := SearchPage{BaseData: middleware.NewBaseData(r)}
	h.pages["foia.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) searchData(w http.ResponseWriter, r *http.Request) {
	userName := strings.TrimSpace(r.URL.Query().Get("user"))
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	contentType := r.URL.Query().Get("type")

	data := SearchPage{
		BaseData: middleware.NewBaseData(r),
		UserName: userName,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Type:     contentType,
		Searched: true,
	}

	// Build query based on content type
	var results []SearchResult
	types := []string{"post", "comment", "message"}
	if contentType != "" {
		types = []string{contentType}
	}

	for _, t := range types {
		var query string
		args := []interface{}{}
		argIdx := 1

		switch t {
		case "post":
			query = `SELECT 'post', p.content, p.user_id, CONCAT(u.first_name, ' ', u.last_name), p.created_at
				FROM posts p JOIN users u ON u.id = p.user_id WHERE 1=1`
		case "comment":
			query = `SELECT 'comment', c.content, c.user_id, CONCAT(u.first_name, ' ', u.last_name), c.created_at
				FROM comments c JOIN users u ON u.id = c.user_id WHERE 1=1`
		case "message":
			query = `SELECT 'message', m.content, m.sender_id, CONCAT(u.first_name, ' ', u.last_name), m.created_at
				FROM messages m JOIN users u ON u.id = m.sender_id WHERE 1=1`
		}

		if userName != "" {
			query += fmt.Sprintf(` AND (LOWER(u.first_name) LIKE LOWER($%d) OR LOWER(u.last_name) LIKE LOWER($%d))`, argIdx, argIdx)
			args = append(args, "%"+userName+"%")
			argIdx++
		}
		if dateFrom != "" {
			query += fmt.Sprintf(` AND %s.created_at >= $%d`, string(t[0]), argIdx)
			args = append(args, dateFrom)
			argIdx++
		}
		if dateTo != "" {
			query += fmt.Sprintf(` AND %s.created_at <= $%d::date + interval '1 day'`, string(t[0]), argIdx)
			args = append(args, dateTo)
			argIdx++
		}

		query += " ORDER BY created_at DESC LIMIT 100"

		rows, err := h.db.QueryContext(r.Context(), query, args...)
		if err != nil {
			slog.Error("foia search failed", "type", t, "error", err)
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var sr SearchResult
			if err := rows.Scan(&sr.Type, &sr.Content, &sr.AuthorID, &sr.Author, &sr.CreatedAt); err != nil {
				continue
			}
			results = append(results, sr)
		}
	}

	data.Results = results
	h.pages["foia.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) exportData(w http.ResponseWriter, r *http.Request) {
	userName := strings.TrimSpace(r.URL.Query().Get("user"))
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	contentType := r.URL.Query().Get("type")

	type ExportRow struct {
		Type      string    `json:"type"`
		Content   string    `json:"content"`
		Author    string    `json:"author"`
		CreatedAt time.Time `json:"created_at"`
	}

	// Respect the content-type filter: when set to a specific type export only
	// that type; when empty (or "all") export everything.
	types := []string{"post", "comment", "message"}
	if contentType != "" && contentType != "all" {
		types = []string{contentType}
	}

	var rows []ExportRow
	for _, t := range types {
		var query string
		args := []interface{}{}
		argIdx := 1

		switch t {
		case "post":
			query = `SELECT p.content, CONCAT(u.first_name, ' ', u.last_name), p.created_at
				FROM posts p JOIN users u ON u.id = p.user_id WHERE 1=1`
		case "comment":
			query = `SELECT c.content, CONCAT(u.first_name, ' ', u.last_name), c.created_at
				FROM comments c JOIN users u ON u.id = c.user_id WHERE 1=1`
		case "message":
			query = `SELECT m.content, CONCAT(u.first_name, ' ', u.last_name), m.created_at
				FROM messages m JOIN users u ON u.id = m.sender_id WHERE 1=1`
		}

		if userName != "" {
			query += fmt.Sprintf(` AND (LOWER(u.first_name) LIKE LOWER($%d) OR LOWER(u.last_name) LIKE LOWER($%d))`, argIdx, argIdx)
			args = append(args, "%"+userName+"%")
			argIdx++
		}
		if dateFrom != "" {
			query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
			args = append(args, dateFrom)
			argIdx++
		}
		if dateTo != "" {
			query += fmt.Sprintf(` AND created_at <= $%d::date + interval '1 day'`, argIdx)
			args = append(args, dateTo)
			argIdx++
		}

		query += " ORDER BY created_at DESC"

		dbRows, err := h.db.QueryContext(r.Context(), query, args...)
		if err != nil {
			continue
		}
		defer dbRows.Close()

		for dbRows.Next() {
			var er ExportRow
			er.Type = t
			if err := dbRows.Scan(&er.Content, &er.Author, &er.CreatedAt); err != nil {
				continue
			}
			rows = append(rows, er)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="foia_export.json"`)
	json.NewEncoder(w).Encode(rows)
}
