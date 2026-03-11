package user

import(
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/connection"
)

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	conn  *connection.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, conn *connection.Handler) *Handler {
	return &Handler { db: db, pages: pages, conn: conn }
}

func (h *Handler) getProfileData(r *http.Request, profileUserID string, currentUserID string) (*ProfileData, error) {
	var user User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, headline, about, avatar_url, location, created_at
		 FROM users WHERE id = $1`, profileUserID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Headline, &user.About, &user.AvatarURL, &user.Location, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	experiences, err := h.getExperiences(r, profileUserID)
	if err != nil {
		return nil, err
	}

	educations, err := h.getEducations(r, profileUserID)
	if err != nil {
		return nil, err
	}

	skills, err := h.getSkills(r, profileUserID)
	if err != nil {
		return nil, err
	}

	return &ProfileData {
		UserID:       currentUserID,
		User:         user,
		Experiences:  experiences,
		Educations:   educations,
		Skills:       skills,
		IsOwnProfile: profileUserID == currentUserID,
	}, nil
}

func (h *Handler) getExperiences(r *http.Request, userID string) ([]Experience, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, title, company, location, start_date, end_date, description
		 FROM experiences WHERE user_id = $1 ORDER BY start_date DESC`, userID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var experiences []Experience
	for rows.Next() {
		var e Experience
		if err := rows.Scan(&e.ID, &e.Title, &e.Company, &e.Location, &e.StartDate, &e.EndDate, &e.Description); err != nil {
			return nil, err
		}

		experiences = append(experiences, e)
	}

	return experiences, rows.Err()
}

func (h *Handler) getEducations(r *http.Request, userID string) ([]Education, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, school, degree, field_of_study, start_year, end_year
		 FROM educations WHERE user_id = $1 ORDER BY start_year DESC`, userID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var educations []Education
	for rows.Next() {
		var e Education
		if err := rows.Scan(&e.ID, &e.School, &e.Degree, &e.FieldOfStudy, &e.StartYear, &e.EndYear); err != nil {
			return nil, err
		}
		educations = append(educations, e)
	}
	return educations, rows.Err()
}

func (h *Handler) getSkills(r *http.Request, userID string) ([]Skill, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name FROM skills WHERE user_id = $1 ORDER BY name`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}

		skills = append(skills, s)
	}

	return skills, rows.Err()
}

func (h *Handler) showOwnProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	http.Redirect(w, r, "/profile/" + userID, http.StatusSeeOther)
}

func (h *Handler) showProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("id")
	currentUserID := middleware.GetUserID(r.Context())

	data, err := h.getProfileData(r, profileID, currentUserID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to load profile", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	status, err := h.conn.GetConnectionStatus(currentUserID, profileID)
	if err != nil {
		slog.Error("failed to load connection status", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data.ConnectionStatus = status

	if err := h.pages["profile.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render profile edit", "error", err)
	}
}

func (h *Handler) showEditProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	data, err := h.getProfileData(r, userID, userID)
	if err != nil {
		slog.Error("failed to load profile for edit", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.pages["profile_edit.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render profile edit", "error", err)
	}
}

func (h *Handler) handleEditProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	headline := strings.TrimSpace(r.FormValue("headline"))
	about := strings.TrimSpace(r.FormValue("about"))
	location := strings.TrimSpace(r.FormValue("location"))
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE users SET first_name = $1, last_name = $2, headline = $3, about = $4, location = $5, updated_at = NOW()
		 WHERE id = $6`,
		 firstName, lastName, headline, about, location, userID,
	)
	if err != nil {
		slog.Error("failed to update profile", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /profile/edit", requireAuth(http.HandlerFunc(h.showEditProfile)))
	mux.Handle("POST /profile/edit", requireAuth(http.HandlerFunc(h.handleEditProfile)))
	mux.Handle("GET /profile/me", requireAuth(http.HandlerFunc(h.showOwnProfile)))
	mux.Handle("GET /profile/{id}", requireAuth(http.HandlerFunc(h.showProfile)))
}
