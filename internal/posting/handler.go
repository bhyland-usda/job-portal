package posting

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/attachment"
	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type PostingAttachment struct {
	ID           string
	OriginalName string
	ContentType  string
	FileSize     int
	Category     string
}

type Posting struct {
	ID            string
	AuthorID      string
	AuthorName    string
	Title         string
	Description   string
	Type          string
	Location      string
	Department    string
	Status        string
	Skills        []string
	MatchCount    int
	CreatedAt     time.Time
	Attachments   []PostingAttachment
	Outcome       string
	OutcomeStatus string
	HasOutcome    bool
	CompletedAt   time.Time
}

type PostingPage struct {
	middleware.BaseData
	Posting    Posting
	Bookmarked bool
}

type SearchPage struct {
	middleware.BaseData
	Query    string
	Type     string
	Postings []Posting
}

type CreatePage struct {
	middleware.BaseData
	Error string
}

type Application struct {
	ID            string
	PostingID     string
	ApplicantID   string
	ApplicantName string
	CoverLetter   string
	Status        string
	CreatedAt     time.Time
}

type ApplicationsPage struct {
	middleware.BaseData
	Posting      Posting
	Applications []Application
}

type ApplyPage struct {
	middleware.BaseData
	Posting Posting
	Error   string
}

type OutcomePage struct {
	middleware.BaseData
	Posting Posting
	Error   string
}

// HistoryEntry is a posting the current user was accepted to, with the
// posting's recorded outcome (if any).
type HistoryEntry struct {
	PostingID     string
	Title         string
	Department    string
	Type          string
	Status        string
	CreatedAt     time.Time
	AcceptedAt    time.Time
	Outcome       string
	OutcomeStatus string
	HasOutcome    bool
	CompletedAt   time.Time
}

type HistoryPage struct {
	middleware.BaseData
	Entries []HistoryEntry
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireManager func(http.Handler) http.Handler) {
	mux.Handle("GET /postings/create", requireAuth(requireManager(http.HandlerFunc(h.showCreate))))
	mux.Handle("POST /postings/create", requireAuth(requireManager(http.HandlerFunc(h.handleCreate))))
	mux.Handle("GET /postings/search", requireAuth(http.HandlerFunc(h.searchPostings)))
	mux.Handle("GET /postings/history", requireAuth(http.HandlerFunc(h.showHistory)))
	mux.Handle("GET /postings/{id}/outcome", requireAuth(http.HandlerFunc(h.showOutcome)))
	mux.Handle("POST /postings/{id}/outcome", requireAuth(http.HandlerFunc(h.handleOutcome)))
	mux.Handle("GET /postings/{id}/apply", requireAuth(http.HandlerFunc(h.showApply)))
	mux.Handle("POST /postings/{id}/apply", requireAuth(http.HandlerFunc(h.handleApply)))
	mux.Handle("GET /postings/{id}/applications", requireAuth(requireManager(http.HandlerFunc(h.showApplications))))
	mux.Handle("POST /postings/applications/{id}/status", requireAuth(requireManager(http.HandlerFunc(h.updateApplicationStatus))))
	mux.Handle("GET /posting-attachments/{id}", requireAuth(http.HandlerFunc(h.serveAttachment)))
	mux.Handle("GET /postings/{id}", requireAuth(http.HandlerFunc(h.showPosting)))
	mux.Handle("POST /postings/{id}/close", requireAuth(requireManager(http.HandlerFunc(h.closePosting))))
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	data := CreatePage{BaseData: middleware.NewBaseData(r)}
	h.pages["posting_create.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 30<<20) // 30MB total
	if err := r.ParseMultipartForm(30 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	postingType := r.FormValue("type")
	location := strings.TrimSpace(r.FormValue("location"))
	department := strings.TrimSpace(r.FormValue("department"))
	skillsRaw := strings.TrimSpace(r.FormValue("skills"))

	if title == "" || description == "" || postingType == "" {
		data := CreatePage{
			BaseData: middleware.NewBaseData(r),
			Error:    "Title, description, and type are required",
		}
		h.pages["posting_create.html"].ExecuteTemplate(w, "base", data)
		return
	}

	validTypes := map[string]bool{"project": true, "detail": true}
	if !validTypes[postingType] {
		http.Error(w, "Invalid posting type", http.StatusBadRequest)
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var postingID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO postings (author_id, title, description, type, location, department)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		userID, title, description, postingType, location, department,
	).Scan(&postingID)
	if err != nil {
		slog.Error("failed to create posting", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if skillsRaw != "" {
		for _, sk := range strings.Split(skillsRaw, ",") {
			skill := strings.TrimSpace(sk)
			if skill == "" {
				continue
			}
			_, err := tx.ExecContext(r.Context(),
				`INSERT INTO posting_skills (posting_id, skill_name)
				 VALUES ($1, $2)
				 ON CONFLICT DO NOTHING`,
				postingID, skill,
			)
			if err != nil {
				slog.Error("failed to add posting skill", "error", err)
			}
		}
	}

	// Handle file attachments (documents and images for postings)
	if r.MultipartForm != nil {
		files := r.MultipartForm.File["attachments"]
		for _, fh := range files {
			file, err := fh.Open()
			if err != nil {
				continue
			}
			validated, err := attachment.ValidateFile(file, fh, attachment.ContextPosting)
			file.Close()
			if err != nil {
				slog.Error("posting attachment validation failed", "error", err)
				continue
			}
			if _, err := tx.ExecContext(r.Context(),
				`INSERT INTO posting_attachments (posting_id, file_data, original_name, content_type, file_size, file_category)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				postingID, validated.Data, validated.OriginalName, validated.ContentType, validated.FileSize, string(validated.Category),
			); err != nil {
				slog.Error("failed to insert posting attachment", "error", err)
				tx.Rollback()
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit posting", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/postings/"+postingID, http.StatusSeeOther)
}

func (h *Handler) showPosting(w http.ResponseWriter, r *http.Request) {
	postingID := r.PathValue("id")

	var posting Posting
	var location, department, outcome, outcomeStatus sql.NullString
	var completedAt sql.NullTime
	err := h.db.QueryRowContext(r.Context(),
		`SELECT p.id, p.author_id,
			CONCAT(u.first_name, ' ', u.last_name),
			p.title, p.description, p.type,
			p.location, p.department, p.status, p.created_at,
			p.outcome, p.outcome_status, p.completed_at
		 FROM postings p
		 JOIN users u on u.id = p.author_id
		 WHERE p.id = $1`,
		postingID,
	).Scan(&posting.ID, &posting.AuthorID, &posting.AuthorName, &posting.Title,
		&posting.Description, &posting.Type, &location, &department,
		&posting.Status, &posting.CreatedAt,
		&outcome, &outcomeStatus, &completedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if location.Valid {
		posting.Location = location.String
	}
	if department.Valid {
		posting.Department = department.String
	}
	if outcome.Valid && strings.TrimSpace(outcome.String) != "" {
		posting.Outcome = outcome.String
		posting.HasOutcome = true
	}
	if outcomeStatus.Valid {
		posting.OutcomeStatus = outcomeStatus.String
	}
	if completedAt.Valid {
		posting.CompletedAt = completedAt.Time
	}

	// Load skills
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT skill_name FROM posting_skills WHERE posting_id = $1 ORDER BY skill_name`,
		postingID,
	)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var skill string
			if err := rows.Scan(&skill); err == nil {
				posting.Skills = append(posting.Skills, skill)
			}
		}
	}

	// Match count - how many employees have at least one matching skill
	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(DISTINCT s.user_id)
		 FROM skills s
		 JOIN posting_skills ps ON LOWER(ps.skill_name) = LOWER(s.name)
		 WHERE ps.posting_id = $1`,
		postingID,
	).Scan(&posting.MatchCount)

	// Load attachments
	attRows, err := h.db.QueryContext(r.Context(),
		`SELECT id, original_name, content_type, file_size, COALESCE(file_category, 'document')
		 FROM posting_attachments WHERE posting_id = $1`, postingID,
	)
	if err == nil {
		for attRows.Next() {
			var a PostingAttachment
			if err := attRows.Scan(&a.ID, &a.OriginalName, &a.ContentType, &a.FileSize, &a.Category); err == nil {
				posting.Attachments = append(posting.Attachments, a)
			}
		}
		attRows.Close()
	}

	var bookmarked bool
	userID := middleware.GetUserID(r.Context())
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id = $1 AND target_type = 'posting' AND target_id = $2)`,
		userID, postingID,
	).Scan(&bookmarked)

	data := PostingPage{
		BaseData:   middleware.NewBaseData(r),
		Posting:    posting,
		Bookmarked: bookmarked,
	}

	h.pages["posting_view.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) serveAttachment(w http.ResponseWriter, r *http.Request) {
	attachID := r.PathValue("id")

	var data []byte
	var contentType, originalName, fileCategory string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT file_data, content_type, original_name, COALESCE(file_category, 'document')
		 FROM posting_attachments WHERE id = $1`,
		attachID,
	).Scan(&data, &contentType, &originalName, &fileCategory)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	attachment.ServeHeaders(w, contentType, originalName, attachment.Category(fileCategory))
	w.Write(data)
}

func (h *Handler) closePosting(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postingID := r.PathValue("id")

	// Only the author can close their posting
	var authorID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT author_id FROM postings WHERE id = $1`,
		postingID,
	).Scan(&authorID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if authorID != userID {
		// Allow admins to close any posting
		role := middleware.GetUserRole(h.db, r.Context(), userID)
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE postings SET status = 'closed', updated_at = NOW() WHERE id = $1`,
		postingID,
	)
	if err != nil {
		slog.Error("failed to close posting", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/postings/"+postingID, http.StatusSeeOther)
}

// showOutcome renders the form the posting author (or an admin) uses to record
// the outcome of a detail/project.
func (h *Handler) showOutcome(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postingID := r.PathValue("id")

	var authorID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT author_id FROM postings WHERE id = $1`,
		postingID,
	).Scan(&authorID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if authorID != userID {
		role := middleware.GetUserRole(h.db, r.Context(), userID)
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	p, err := h.loadPosting(r.Context(), postingID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := OutcomePage{BaseData: middleware.NewBaseData(r), Posting: p}
	h.pages["posting_outcome.html"].ExecuteTemplate(w, "base", data)
}

// handleOutcome saves the recorded outcome, sets completed_at, closes the
// posting, and redirects back to the posting view. Only the author or an admin
// may record an outcome.
func (h *Handler) handleOutcome(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postingID := r.PathValue("id")

	var authorID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT author_id FROM postings WHERE id = $1`,
		postingID,
	).Scan(&authorID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if authorID != userID {
		role := middleware.GetUserRole(h.db, r.Context(), userID)
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	outcome := strings.TrimSpace(r.FormValue("outcome"))
	outcomeStatus := r.FormValue("outcome_status")

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE postings SET outcome = $1, outcome_status = $2, completed_at = NOW(), status = 'closed', updated_at = NOW() WHERE id = $3`,
		outcome, outcomeStatus, postingID,
	)
	if err != nil {
		slog.Error("failed to record outcome", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/postings/"+postingID, http.StatusSeeOther)
}

// showHistory lists the postings the current user was accepted to (their detail
// history), along with any recorded outcome for each posting.
func (h *Handler) showHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT p.id, p.title, COALESCE(p.department, ''), p.type, p.status,
			p.created_at, pa.updated_at,
			p.outcome, p.outcome_status, p.completed_at
		 FROM posting_applications pa
		 JOIN postings p ON p.id = pa.posting_id
		 WHERE pa.applicant_id = $1 AND pa.status = 'accepted'
		 ORDER BY pa.updated_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load detail history", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var outcome, outcomeStatus sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&e.PostingID, &e.Title, &e.Department, &e.Type, &e.Status,
			&e.CreatedAt, &e.AcceptedAt, &outcome, &outcomeStatus, &completedAt); err != nil {
			slog.Error("failed to scan history entry", "error", err)
			continue
		}
		if outcome.Valid && strings.TrimSpace(outcome.String) != "" {
			e.Outcome = outcome.String
			e.HasOutcome = true
		}
		if outcomeStatus.Valid {
			e.OutcomeStatus = outcomeStatus.String
		}
		if completedAt.Valid {
			e.CompletedAt = completedAt.Time
		}
		entries = append(entries, e)
	}

	data := HistoryPage{
		BaseData: middleware.NewBaseData(r),
		Entries:  entries,
	}
	h.pages["posting_history.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) searchPostings(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	postingType := r.URL.Query().Get("type")

	data := SearchPage{
		BaseData: middleware.NewBaseData(r),
		Query:    query,
		Type:     postingType,
	}

	args := []interface{}{}
	conditions := []string{"p.status = 'active'"}
	argIdx := 1

	if query != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(p.title) LIKE LOWER($%d) OR LOWER(p.description) LIKE LOWER($%d) OR EXISTS(SELECT 1 FROM posting_skills ps WHERE ps.posting_id = p.id AND LOWER(ps.skill_name) LIKE LOWER($%d)))",
			argIdx, argIdx, argIdx))
		args = append(args, "%"+query+"%")
		argIdx++
	}

	if postingType != "" {
		conditions = append(conditions, fmt.Sprintf("p.type = $%d", argIdx))
		args = append(args, postingType)
		argIdx++
	}

	sql := fmt.Sprintf(`SELECT p.id, p.author_id,
			CONCAT(u.first_name, ' ', u.last_name),
			p.title, p.description, p.type,
			COALESCE(p.location, ''), COALESCE(p.department, ''),
			p.status, p.created_at
		FROM postings p
		JOIN users u ON u.id = p.author_id
		WHERE %s
		ORDER BY p.created_at DESC
		LIMIT 50`, strings.Join(conditions, " AND "))

	rows, err := h.db.QueryContext(r.Context(), sql, args...)
	if err != nil {
		slog.Error("failed to search postings", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var p Posting
			if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Title,
				&p.Description, &p.Type, &p.Location, &p.Department,
				&p.Status, &p.CreatedAt); err != nil {
				continue
			}
			data.Postings = append(data.Postings, p)
		}
	}

	h.pages["posting_search.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) loadPosting(ctx context.Context, postingID string) (Posting, error) {
	var p Posting
	var location, department sql.NullString
	err := h.db.QueryRowContext(ctx,
		`SELECT p.id, p.author_id,
			CONCAT(u.first_name, ' ', u.last_name),
			p.title, p.description, p.type,
			p.location, p.department, p.status, p.created_at
		 FROM postings p
		 JOIN users u ON u.id = p.author_id
		 WHERE p.id = $1`,
		postingID,
	).Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Title,
		&p.Description, &p.Type, &location, &department,
		&p.Status, &p.CreatedAt)
	if err != nil {
		return p, err
	}
	if location.Valid {
		p.Location = location.String
	}
	if department.Valid {
		p.Department = department.String
	}
	return p, nil
}

func (h *Handler) showApply(w http.ResponseWriter, r *http.Request) {
	postingID := r.PathValue("id")

	p, err := h.loadPosting(r.Context(), postingID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if p.Status != "active" {
		http.Error(w, "This posting is no longer accepting applications", http.StatusBadRequest)
		return
	}

	data := ApplyPage{BaseData: middleware.NewBaseData(r), Posting: p}
	h.pages["posting_apply.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) handleApply(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postingID := r.PathValue("id")

	p, err := h.loadPosting(r.Context(), postingID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if p.Status != "active" {
		http.Error(w, "This posting is no longer accepting applications", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	coverLetter := strings.TrimSpace(r.FormValue("cover_letter"))
	if coverLetter == "" {
		data := ApplyPage{
			BaseData: middleware.NewBaseData(r),
			Posting:  p,
			Error:    "Cover letter is required",
		}
		h.pages["posting_apply.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO posting_applications (posting_id, applicant_id, cover_letter)
		 VALUES ($1, $2, $3)`,
		postingID, userID, coverLetter,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			data := ApplyPage{
				BaseData: middleware.NewBaseData(r),
				Posting:  p,
				Error:    "You have already applied to this posting",
			}
			h.pages["posting_apply.html"].ExecuteTemplate(w, "base", data)
			return
		}
		slog.Error("failed to submit application", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/postings/"+postingID, http.StatusSeeOther)
}

func (h *Handler) showApplications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postingID := r.PathValue("id")

	p, err := h.loadPosting(r.Context(), postingID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Only author or admin can view applications
	if p.AuthorID != userID {
		role := middleware.GetUserRole(h.db, r.Context(), userID)
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT pa.id, pa.posting_id, pa.applicant_id,
			CONCAT(u.first_name, ' ', u.last_name),
			pa.cover_letter, pa.status, pa.created_at
		 FROM posting_applications pa
		 JOIN users u ON u.id = pa.applicant_id
		 WHERE pa.posting_id = $1
		 ORDER BY pa.created_at DESC`,
		postingID,
	)
	if err != nil {
		slog.Error("failed to load applications", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.PostingID, &a.ApplicantID,
			&a.ApplicantName, &a.CoverLetter, &a.Status, &a.CreatedAt); err != nil {
			slog.Error("failed to scan application", "error", err)
			continue
		}
		apps = append(apps, a)
	}

	data := ApplicationsPage{
		BaseData:     middleware.NewBaseData(r),
		Posting:      p,
		Applications: apps,
	}
	h.pages["posting_applications.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) updateApplicationStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	appID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	newStatus := r.FormValue("status")
	validStatuses := map[string]bool{"shortlisted": true, "accepted": true, "rejected": true}
	if !validStatuses[newStatus] {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	// Verify the current user is the posting author or admin
	var postingID, authorID, applicantID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT pa.posting_id, p.author_id, pa.applicant_id
		 FROM posting_applications pa
		 JOIN postings p ON p.id = pa.posting_id
		 WHERE pa.id = $1`,
		appID,
	).Scan(&postingID, &authorID, &applicantID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if authorID != userID {
		role := middleware.GetUserRole(h.db, r.Context(), userID)
		if role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE posting_applications SET status = $1, updated_at = NOW()
		 WHERE id = $2`,
		newStatus, appID,
	)
	if err != nil {
		slog.Error("failed to update application status", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// An accepted application means the applicant landed a detail — check the
	// Detail Completed badge.
	if newStatus == "accepted" {
		badge.CheckAndAward(r.Context(), h.db, applicantID)
	}

	http.Redirect(w, r, "/postings/"+postingID+"/applications", http.StatusSeeOther)
}

func GetMatchedPostings(db *sql.DB, ctx context.Context, userID string) ([]Posting, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT p.id, p.author_id,
			CONCAT(u.first_name, ' ', u.last_name),
			p.title, p.description, p.type,
			COALESCE(p.location, ''), COALESCE(p.department, ''),
			p.status, p.created_at
		FROM postings p
		JOIN users u ON u.id = p.author_id
		WHERE p.status = 'active'
			AND EXISTS(
				SELECT 1 FROM posting_skills ps
				JOIN skills s ON LOWER(s.name) = LOWER(ps.skill_name)
				WHERE ps.posting_id = p.id AND s.user_id = $1
			) 
		ORDER BY p.created_at DESC
		LIMIT 50`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query postings: %w", err)
	}
	defer rows.Close()

	var postings []Posting
	for rows.Next() {
		var posting Posting
		if err := rows.Scan(&posting.ID, &posting.AuthorID, &posting.AuthorName, &posting.Title,
			&posting.Description, &posting.Type, &posting.Location, &posting.Department,
			&posting.Status, &posting.CreatedAt); err != nil {
			continue
		}
		postings = append(postings, posting)
	}
	return postings, rows.Err()
}
