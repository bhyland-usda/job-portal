package certification

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// expiringSoonWindow is how far ahead (in days) a certification is considered
// "Expiring soon".
const expiringSoonWindow = 90 * 24 * time.Hour

type Certification struct {
	ID        string
	Name      string
	Issuer    string
	IssuedOn  sql.NullTime
	ExpiresOn sql.NullTime
	CreatedAt time.Time
	Status    string
}

type CertificationsPage struct {
	middleware.BaseData
	Certifications []Certification
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /certifications", requireAuth(http.HandlerFunc(h.showCertifications)))
	mux.Handle("POST /certifications", requireAuth(http.HandlerFunc(h.handleCreate)))
	mux.Handle("POST /certifications/{id}/delete", requireAuth(http.HandlerFunc(h.handleDelete)))
}

// computeStatus derives the human-readable status badge for a certification
// based on its expiry date relative to now.
func computeStatus(expires sql.NullTime, now time.Time) string {
	if !expires.Valid {
		return "Active"
	}
	if expires.Time.Before(now) {
		return "Expired"
	}
	if expires.Time.Sub(now) <= expiringSoonWindow {
		return "Expiring soon"
	}
	return "Active"
}

func (h *Handler) getUserCertifications(ctx context.Context, userID string) ([]Certification, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, name, issuer, issued_on, expires_on, created_at
		 FROM certifications
		 WHERE user_id = $1
		 ORDER BY expires_on ASC NULLS LAST, created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query certifications: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var certs []Certification
	for rows.Next() {
		var c Certification
		var issuer sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &issuer, &c.IssuedOn, &c.ExpiresOn, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan certification: %w", err)
		}
		c.Issuer = issuer.String
		c.Status = computeStatus(c.ExpiresOn, now)
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

func (h *Handler) showCertifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	certs, err := h.getUserCertifications(r.Context(), userID)
	if err != nil {
		slog.Error("failed to load certifications", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := CertificationsPage{
		BaseData:       middleware.NewBaseData(r),
		Certifications: certs,
	}

	h.pages["certifications.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		// Name is required; nothing to insert, return to the list.
		http.Redirect(w, r, "/certifications", http.StatusSeeOther)
		return
	}

	issuer := nullString(r.FormValue("issuer"))
	issuedOn := nullDate(r.FormValue("issued_on"))
	expiresOn := nullDate(r.FormValue("expires_on"))

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO certifications (user_id, name, issuer, issued_on, expires_on)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, name, issuer, issuedOn, expiresOn,
	)
	if err != nil {
		slog.Error("failed to create certification", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/certifications", http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	certID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM certifications WHERE id = $1 AND user_id = $2`,
		certID, userID,
	)
	if err != nil {
		slog.Error("failed to delete certification", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/certifications", http.StatusSeeOther)
}

// nullString trims the value and returns a NULL-able string (empty -> NULL).
func nullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: v, Valid: true}
}

// nullDate parses an HTML date input (YYYY-MM-DD). Empty or unparseable
// values become NULL.
func nullDate(v string) sql.NullTime {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullTime{Valid: false}
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}
