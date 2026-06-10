package resume

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Resume struct {
	ID           string
	OriginalName string
	ContentType  string
	FileSize     int
	CreatedAt    string
}

type ResumePage struct {
	middleware.BaseData
	Resumes []Resume
	Error   string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /resumes", requireAuth(http.HandlerFunc(h.showResumes)))
	mux.Handle("POST /resumes/upload", requireAuth(http.HandlerFunc(h.handleUpload)))
	mux.Handle("GET /resumes/{id}/download", requireAuth(http.HandlerFunc(h.handleDownload)))
	mux.Handle("POST /resumes/{id}/delete", requireAuth(http.HandlerFunc(h.handleDelete)))
	mux.Handle("GET /resumes/{id}/delete", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
	})))
	mux.Handle("GET /resumes/generate", requireAuth(http.HandlerFunc(h.generateResume)))
}

func (h *Handler) showResumes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	resumes, err := h.getUserResumes(r, userID)
	if err != nil {
		slog.Error("failed to load resumes", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := ResumePage{
		BaseData: middleware.NewBaseData(r),
		Resumes:  resumes,
	}

	h.pages["resumes.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) getUserResumes(r *http.Request, userID string) ([]Resume, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, original_name, content_type, file_size, created_at
		 FROM resumes
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query resumes: %w", err)
	}
	defer rows.Close()

	var resumes []Resume
	for rows.Next() {
		var res Resume
		if err := rows.Scan(&res.ID, &res.OriginalName, &res.ContentType,
			&res.FileSize, &res.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan resume: %w", err)
		}
		resumes = append(resumes, res)
	}
	return resumes, rows.Err()
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Check count limit
	var count int
	err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM resumes WHERE user_id = $1`,
		userID,
	).Scan(&count)
	if err != nil {
		slog.Error("failed to check resume count", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if count >= 5 {
		http.Error(w, "Maximum 5 resumes allowed. Delete one before uploading.", http.StatusBadRequest)
		return
	}

	// 10MB limit for resumes
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large (max 10MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("resume")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate content type via magic bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}

	contentType := http.DetectContentType(buf[:n])
	allowedTypes := map[string]bool{
		"application/pdf": true,
	}

	if !allowedTypes[contentType] {
		http.Error(w, "Only PDF files are allowed", http.StatusBadRequest)
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Failed to process file", http.StatusInternalServerError)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Sanitize original file name
	originalName := strings.TrimSpace(header.Filename)
	if originalName == "" {
		originalName = "resume.pdf"
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO resumes (user_id, original_name, file_data, content_type, file_size)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, originalName, data, contentType, len(data),
	)
	if err != nil {
		slog.Error("failed to save resume", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/resumes", http.StatusSeeOther)
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	resumeID := r.PathValue("id")

	var data []byte
	var contentType, originalName string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT file_data, content_type, original_name
		 FROM resumes
		 WHERE id = $1 AND user_id = $2`,
		resumeID, userID,
	).Scan(&data, &contentType, &originalName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, originalName))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Write(data)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	resumeID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM resumes WHERE id = $1 AND user_id = $2`,
		resumeID, userID,
	)
	if err != nil {
		slog.Error("failed to delete resume", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/resumes", http.StatusSeeOther)
}

func (h *Handler) generateResume(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var firstName, lastName, headline, location, about string
	var avatarURL sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name, COALESCE(headline, ''), COALESCE(location, ''),
			COALESCE(about, ''), avatar_url
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&firstName, &lastName, &headline, &location, &about, &avatarURL)
	if err != nil {
		slog.Error("failed to load user for resume", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Experience
	expRows, err := h.db.QueryContext(r.Context(),
		`SELECT title, company, COALESCE(location, ''), COALESCE(description, ''),
			start_date, end_date
		 FROM experiences WHERE user_id = $1
		 ORDER BY start_date DESC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load experiences", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer expRows.Close()

	type Experience struct {
		Title       string
		Company     string
		Location    string
		Description string
		StartDate   time.Time
		EndDate     sql.NullTime
	}

	var experiences []Experience
	for expRows.Next() {
		var e Experience
		if err := expRows.Scan(&e.Title, &e.Company, &e.Location,
			&e.Description, &e.StartDate, &e.EndDate); err != nil {
			continue
		}
		experiences = append(experiences, e)
	}

	// Education
	eduRows, err := h.db.QueryContext(r.Context(),
		`SELECT school, degree, COALESCE(field_of_study, ''),
			start_year, end_year
		 FROM educations WHERE user_id = $1
		 ORDER BY start_year DESC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load educations", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer eduRows.Close()

	type Education struct {
		School       string
		Degree       string
		FieldOfStudy string
		StartYear    int
		EndYear      sql.NullInt64
	}

	var educations []Education
	for eduRows.Next() {
		var e Education
		if err := eduRows.Scan(&e.School, &e.Degree, &e.FieldOfStudy,
			&e.StartYear, &e.EndYear); err != nil {
			continue
		}
		educations = append(educations, e)
	}

	// Skills
	skillRows, err := h.db.QueryContext(r.Context(),
		`SELECT name from skills WHERE user_id = $1 ORDER BY name`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load skills", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer skillRows.Close()

	var skills []string
	for skillRows.Next() {
		var skill string
		if err := skillRows.Scan(&skill); err != nil {
			continue
		}
		skills = append(skills, skill)
	}

	data := map[string]interface{}{
		"UserID":      userID,
		"FirstName":   firstName,
		"LastName":    lastName,
		"Headline":    headline,
		"Location":    location,
		"About":       about,
		"Experiences": experiences,
		"Educations":  educations,
		"Skills":      skills,
	}

	h.pages["resume_view.html"].ExecuteTemplate(w, "resume_base", data)
}
