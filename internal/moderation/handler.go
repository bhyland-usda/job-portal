package moderation

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// Report is a single user-submitted content report shown in the admin queue.
type Report struct {
	ID           string
	ReporterID   string
	ReporterName string
	ContentType  string
	ContentID    string
	Reason       string
	Status       string
	CreatedAt    time.Time
}

type QueuePage struct {
	middleware.BaseData
	Reports []Report
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	// Any authenticated user can report content.
	mux.Handle("POST /report", requireAuth(http.HandlerFunc(h.createReport)))
	// Admins review and resolve the moderation queue.
	mux.Handle("GET /admin/moderation", requireAuth(requireAdmin(http.HandlerFunc(h.showQueue))))
	mux.Handle("POST /admin/moderation/{id}/resolve", requireAuth(requireAdmin(http.HandlerFunc(h.resolveReport))))
}

// createReport records a content report from the current user. The content_type
// and content_id identify the offending item (e.g. content_type=post); reason is
// free-text and optional.
func (h *Handler) createReport(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	contentType := strings.TrimSpace(r.FormValue("content_type"))
	contentID := strings.TrimSpace(r.FormValue("content_id"))
	reason := strings.TrimSpace(r.FormValue("reason"))

	if contentType == "" || contentID == "" {
		http.Error(w, "content_type and content_id are required", http.StatusBadRequest)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO content_reports (reporter_id, content_type, content_id, reason)
		 VALUES ($1, $2, $3, $4)`,
		userID, contentType, contentID, reason,
	)
	if err != nil {
		slog.Error("failed to create content report", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return the reporter to the feed; the report is queued for admin review.
	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

// showQueue lists pending reports (newest first) with the reporter's name.
func (h *Handler) showQueue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT cr.id, COALESCE(cr.reporter_id::text, ''),
		        COALESCE(u.first_name || ' ' || u.last_name, 'Unknown'),
		        cr.content_type, cr.content_id::text,
		        COALESCE(cr.reason, ''), cr.status, cr.created_at
		 FROM content_reports cr
		 LEFT JOIN users u ON u.id = cr.reporter_id
		 WHERE cr.status = 'pending'
		 ORDER BY cr.created_at DESC
		 LIMIT 100`,
	)
	if err != nil {
		slog.Error("failed to load moderation queue", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var rep Report
		if err := rows.Scan(
			&rep.ID, &rep.ReporterID, &rep.ReporterName,
			&rep.ContentType, &rep.ContentID,
			&rep.Reason, &rep.Status, &rep.CreatedAt,
		); err != nil {
			slog.Error("failed to scan content report", "error", err)
			continue
		}
		reports = append(reports, rep)
	}

	data := QueuePage{
		BaseData: middleware.NewBaseData(r),
		Reports:  reports,
	}

	if err := h.pages["moderation_queue.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render moderation queue", "error", err)
	}
}

// resolveReport marks a report 'reviewed' or 'dismissed' and stamps the acting
// admin as reviewed_by. Any action other than 'dismissed' resolves to 'reviewed'.
func (h *Handler) resolveReport(w http.ResponseWriter, r *http.Request) {
	adminID := middleware.GetUserID(r.Context())
	reportID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	status := "reviewed"
	if strings.TrimSpace(r.FormValue("action")) == "dismissed" {
		status = "dismissed"
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE content_reports SET status = $1, reviewed_by = $2 WHERE id = $3`,
		status, adminID, reportID,
	)
	if err != nil {
		slog.Error("failed to resolve content report", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/moderation", http.StatusSeeOther)
}
