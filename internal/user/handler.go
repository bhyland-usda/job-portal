package user

import (
	"context"
	"database/sql"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/connection"
	"github.com/bhyland-usda/job-portal/internal/follow"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/verification"
)

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	conn  *connection.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, conn *connection.Handler) *Handler {
	return &Handler{db: db, pages: pages, conn: conn}
}

func (h *Handler) getProfileData(r *http.Request, profileUserID string, currentUserID string) (*ProfileData, error) {
	var user User
	var birthday, hireDate, workStatusUntil sql.NullTime
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, headline, about, avatar_url, location, created_at, birthday, hire_date, work_status, work_status_until, profile_visibility
		 FROM users WHERE id = $1`, profileUserID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Headline, &user.About, &user.AvatarURL, &user.Location, &user.CreatedAt, &birthday, &hireDate, &user.WorkStatus, &workStatusUntil, &user.ProfileVisibility)

	if err != nil {
		return nil, err
	}

	if birthday.Valid {
		b := birthday.Time
		user.Birthday = &b
	}
	if hireDate.Valid {
		hd := hireDate.Time
		user.HireDate = &hd
	}
	if workStatusUntil.Valid {
		wu := workStatusUntil.Time
		user.WorkStatusUntil = &wu
	}

	experiences, err := h.getExperiences(r, profileUserID)
	if err != nil {
		return nil, err
	}

	educations, err := h.getEducations(r, profileUserID)
	if err != nil {
		return nil, err
	}

	skills, err := h.getSkillsWithEndorsements(r, profileUserID, currentUserID)
	if err != nil {
		return nil, err
	}

	data := &ProfileData{
		BaseData:     middleware.NewBaseData(r),
		User:         user,
		Experiences:  experiences,
		Educations:   educations,
		Skills:       skills,
		IsOwnProfile: profileUserID == currentUserID,
	}

	if data.IsOwnProfile {
		c := GetProfileCompleteness(r.Context(), h.db, currentUserID)
		data.Completeness = &c
	}

	return data, nil
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

func (h *Handler) showAddExperience(w http.ResponseWriter, r *http.Request) {
	data := ExperienceFormData{
		BaseData: middleware.NewBaseData(r),
	}

	if err := h.pages["experience_form.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render experience form", "error", err)
	}
}

func (h *Handler) handleAddExperience(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	company := strings.TrimSpace(r.FormValue("company"))
	location := strings.TrimSpace(r.FormValue("location"))
	startStr := r.FormValue("start_date")
	endStr := r.FormValue("end_date")
	description := strings.TrimSpace(r.FormValue("description"))

	if title == "" || company == "" || startStr == "" {
		data := ExperienceFormData{
			BaseData: middleware.NewBaseData(r),
			Experience: Experience{
				Title:       title,
				Company:     company,
				Location:    location,
				Description: description,
			},
			Error: "Title, company, and start date are required.",
		}
		h.pages["experience_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	var endDate *time.Time
	if endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			http.Error(w, "Invalid end date", http.StatusBadRequest)
			return
		}
		endDate = &t
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO experiences (user_id, title, company, location, start_date, end_date, description)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, title, company, location, startDate, endDate, description,
	)
	if err != nil {
		slog.Error("failed to add experience", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) showEditExperience(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	expID := r.PathValue("id")

	var exp Experience
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, title, company, location, start_date, end_date, description
		 FROM experiences WHERE id = $1 AND user_id = $2`,
		expID, userID,
	).Scan(&exp.ID, &exp.Title, &exp.Company, &exp.Location, &exp.StartDate, &exp.EndDate, &exp.Description)

	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to load experience", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := ExperienceFormData{
		BaseData:   middleware.NewBaseData(r),
		Experience: exp,
	}

	if err := h.pages["experience_form.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render experience form", "error", err)
	}
}

func (h *Handler) handleEditExperience(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	expID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	company := strings.TrimSpace(r.FormValue("company"))
	location := strings.TrimSpace(r.FormValue("location"))
	startStr := r.FormValue("start_date")
	endStr := r.FormValue("end_date")
	description := strings.TrimSpace(r.FormValue("description"))

	if title == "" || company == "" || startStr == "" {
		data := ExperienceFormData{
			BaseData: middleware.NewBaseData(r),
			Experience: Experience{
				ID:          expID,
				Title:       title,
				Company:     company,
				Location:    location,
				Description: description,
			},
			Error: "Title, company, and start date are required.",
		}
		h.pages["experience_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	var endDate *time.Time
	if endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			http.Error(w, "Invalid end date", http.StatusBadRequest)
			return
		}
		endDate = &t
	}

	result, err := h.db.ExecContext(r.Context(),
		`UPDATE experiences SET title = $1, company = $2, location = $3,
                 start_date = $4, end_date = $5, description = $6
                 WHERE id = $7 AND user_id = $8`,
		title, company, location, startDate, endDate, description, expID, userID,
	)
	if err != nil {
		slog.Error("failed to update experience", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) handleDeleteExperience(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	expID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM experiences WHERE id = $1 AND user_id = $2`,
		expID, userID,
	)
	if err != nil {
		slog.Error("failed to delete experience", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
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

func (h *Handler) showAddEducation(w http.ResponseWriter, r *http.Request) {
	data := EducationFormData{
		BaseData: middleware.NewBaseData(r),
	}

	if err := h.pages["education_form.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render education form", "error", err)
	}
}

func (h *Handler) handleAddEducation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	school := strings.TrimSpace(r.FormValue("school"))
	degree := strings.TrimSpace(r.FormValue("degree"))
	fieldOfStudy := strings.TrimSpace(r.FormValue("field_of_study"))
	startStr := r.FormValue("start_year")
	endStr := r.FormValue("end_year")

	if school == "" || startStr == "" {
		data := EducationFormData{
			BaseData: middleware.NewBaseData(r),
			Education: Education{
				School:       school,
				Degree:       degree,
				FieldOfStudy: fieldOfStudy,
			},
			Error: "School and start year are required.",
		}
		h.pages["education_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	startYear, err := strconv.Atoi(startStr)
	if err != nil {
		http.Error(w, "Invalid start year", http.StatusBadRequest)
		return
	}

	var endYear *int
	if endStr != "" {
		year, err := strconv.Atoi(endStr)
		if err != nil {
			http.Error(w, "Invalid end year", http.StatusBadRequest)
			return
		}
		endYear = &year
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO educations (user_id, school, degree, field_of_study, start_year, end_year)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, school, degree, fieldOfStudy, startYear, endYear,
	)
	if err != nil {
		slog.Error("failed to add education", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) showEditEducation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	eduID := r.PathValue("id")

	var edu Education
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, school, degree, field_of_study, start_year, end_year
		 FROM educations WHERE id = $1 AND user_id = $2`,
		eduID, userID,
	).Scan(&edu.ID, &edu.School, &edu.Degree, &edu.FieldOfStudy, &edu.StartYear, &edu.EndYear)

	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to load education", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := EducationFormData{
		BaseData:  middleware.NewBaseData(r),
		Education: edu,
	}

	if err := h.pages["education_form.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render education form", "error", err)
	}
}

func (h *Handler) handleEditEducation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	eduID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	school := strings.TrimSpace(r.FormValue("school"))
	degree := strings.TrimSpace(r.FormValue("degree"))
	fieldOfStudy := strings.TrimSpace(r.FormValue("field_of_study"))
	startStr := r.FormValue("start_year")
	endStr := r.FormValue("end_year")

	if school == "" || startStr == "" {
		data := EducationFormData{
			BaseData: middleware.NewBaseData(r),
			Education: Education{
				ID:           eduID,
				School:       school,
				Degree:       degree,
				FieldOfStudy: fieldOfStudy,
			},
			Error: "School and start year are required.",
		}
		h.pages["education_form.html"].ExecuteTemplate(w, "base", data)
		return
	}

	startYear, err := strconv.Atoi(startStr)
	if err != nil {
		http.Error(w, "Invalid start year", http.StatusBadRequest)
		return
	}

	var endYear *int
	if endStr != "" {
		year, err := strconv.Atoi(endStr)
		if err != nil {
			http.Error(w, "Invalid end year", http.StatusBadRequest)
			return
		}
		endYear = &year
	}

	result, err := h.db.ExecContext(r.Context(),
		`UPDATE educations SET school = $1, degree = $2, field_of_study = $3,
		 start_year = $4, end_year = $5
		 WHERE id = $6 AND user_id = $7`,
		school, degree, fieldOfStudy, startYear, endYear, eduID, userID,
	)
	if err != nil {
		slog.Error("failed to update education", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) handleDeleteEducation(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	eduID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM educations WHERE id = $1 AND user_id = $2`,
		eduID, userID,
	)
	if err != nil {
		slog.Error("failed to delete education", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
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

func (h *Handler) getSkillsWithEndorsements(r *http.Request, profileUserID, viewerID string) ([]Skill, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT s.id, s.name,
			(SELECT COUNT(*) FROM skill_endorsements WHERE skill_id = s.id),
			EXISTS(SELECT 1 FROM skill_endorsements WHERE skill_id = s.id AND endorser_id = $2)
		 FROM skills s
		 WHERE s.user_id = $1
		 ORDER BY s.name`, profileUserID, viewerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.EndorsementCount, &s.EndorsedByUser); err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

func (h *Handler) showSkills(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	skills, err := h.getSkills(r, userID)
	if err != nil {
		slog.Error("failed to load skills", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := SkillsFormData{
		BaseData: middleware.NewBaseData(r),
		Skills:   skills,
	}

	if err := h.pages["skills_form.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render skills form", "error", err)
	}
}

func (h *Handler) handleAddSkill(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, "/profile/skills/edit", http.StatusSeeOther)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO skills (user_id, name) VALUES ($1, $2)
		 ON CONFLICT (user_id, name) DO NOTHING`,
		userID, name,
	)
	if err != nil {
		slog.Error("failed to add skill", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/skills/edit", http.StatusSeeOther)
}

func (h *Handler) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	skillID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM skills WHERE id = $1 AND user_id = $2`,
		skillID, userID,
	)
	if err != nil {
		slog.Error("failed to delete skill", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/skills/edit", http.StatusSeeOther)
}

func (h *Handler) showOwnProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	http.Redirect(w, r, "/profile/"+userID, http.StatusSeeOther)
}

// SkillView decorates a Skill with the verification count for rendering.
// It is a view-only type used to pass per-skill verification data to the
// profile template without changing the persisted Skill model.
type SkillView struct {
	Skill
	VerificationCount int
}

// ProfileView wraps ProfileData with view-only fields that are computed in the
// profile handler (follow state, per-skill verification counts). Embedding
// *ProfileData promotes its fields so the template keeps using .User, .Skills,
// .IsOwnProfile, etc. unchanged.
type ProfileView struct {
	*ProfileData
	IsFollowing  bool
	SkillViews   []SkillView
	PinnedPosts  []PinnedPost
	Celebrations []Celebration
}

// getPinnedPosts loads the posts the given profile owner has pinned, newest
// pinned_at first, joining in the post content and author display fields the
// same way the feed does.
func (h *Handler) getPinnedPosts(r *http.Request, profileUserID string) ([]PinnedPost, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT p.id, p.user_id, u.first_name, u.last_name, u.avatar_url,
		        p.content, p.created_at, pp.pinned_at
		 FROM pinned_profile_posts pp
		 JOIN posts p ON p.id = pp.post_id
		 JOIN users u ON u.id = p.user_id
		 WHERE pp.user_id = $1
		 ORDER BY pp.pinned_at DESC`, profileUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pinned []PinnedPost
	for rows.Next() {
		var p PinnedPost
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorFirstName, &p.AuthorLastName,
			&p.AuthorAvatarURL, &p.Content, &p.CreatedAt, &p.PinnedAt); err != nil {
			return nil, err
		}
		pinned = append(pinned, p)
	}
	return pinned, rows.Err()
}

// getWeeklyCelebrations returns the viewer's accepted connections whose birthday
// or work anniversary (by month/day) falls within the next 7 days, for the
// "Celebrations this week" list on the owner's own profile.
func (h *Handler) getWeeklyCelebrations(r *http.Request, userID string) ([]Celebration, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT u.id, u.first_name, u.last_name, u.avatar_url, u.birthday, u.hire_date
		 FROM users u
		 JOIN connections c ON (
		     (c.requester_id = $1 AND c.addressee_id = u.id) OR
		     (c.addressee_id = $1 AND c.requester_id = u.id)
		 )
		 WHERE c.status = 'accepted'
		   AND (u.birthday IS NOT NULL OR u.hire_date IS NOT NULL)`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Celebration
	for rows.Next() {
		var c Celebration
		var birthday, hireDate sql.NullTime
		if err := rows.Scan(&c.UserID, &c.FirstName, &c.LastName, &c.AvatarURL, &birthday, &hireDate); err != nil {
			return nil, err
		}
		if birthday.Valid && withinNextWeek(birthday.Time) {
			out = append(out, Celebration{
				UserID: c.UserID, FirstName: c.FirstName, LastName: c.LastName,
				AvatarURL: c.AvatarURL, Kind: "birthday",
			})
		}
		if hireDate.Valid && withinNextWeek(hireDate.Time) {
			years := User{HireDate: &hireDate.Time}.AnniversaryYears()
			out = append(out, Celebration{
				UserID: c.UserID, FirstName: c.FirstName, LastName: c.LastName,
				AvatarURL: c.AvatarURL, Kind: "anniversary", Years: years,
			})
		}
	}
	return out, rows.Err()
}

// withinNextWeek reports whether the month/day of d (ignoring the year) falls
// within the 7-day window starting today.
func withinNextWeek(d time.Time) bool {
	now := time.Now()
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, i)
		if day.Month() == d.Month() && day.Day() == d.Day() {
			return true
		}
	}
	return false
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

	// Privacy enforcement: when viewing another user's profile, honor their
	// profile_visibility setting. If the viewer may not see the full profile,
	// drop the loaded detail sections and render the limited view (name + a
	// reason + a connect prompt). Owners and admins always see the full profile.
	viewerRole := middleware.GetUserInfo(r.Context()).Role
	if !CanViewFullProfile(data.User.ProfileVisibility, data.IsOwnProfile, viewerRole, status) {
		data.Restricted = true
		data.LimitedReason = LimitedReason(data.User.ProfileVisibility)
		// Strip anything sensitive so it never reaches the template.
		data.User.About = ""
		data.User.Headline = ""
		data.User.Location = ""
		data.User.Birthday = nil
		data.User.HireDate = nil
		data.Experiences = nil
		data.Educations = nil
		data.Skills = nil

		view := ProfileView{ProfileData: data}
		if err := h.pages["profile.html"].ExecuteTemplate(w, "base", view); err != nil {
			slog.Error("failed to render limited profile", "error", err)
		}
		return
	}

	// Build per-skill verification counts for display (BUG-2).
	skillViews := make([]SkillView, 0, len(data.Skills))
	for _, s := range data.Skills {
		skillViews = append(skillViews, SkillView{
			Skill:             s,
			VerificationCount: verification.GetVerificationCount(h.db, s.ID),
		})
	}

	view := ProfileView{
		ProfileData: data,
		SkillViews:  skillViews,
	}

	// Load the profile owner's pinned posts (rendered at the top of the page).
	pinned, err := h.getPinnedPosts(r, profileID)
	if err != nil {
		slog.Error("failed to load pinned posts", "error", err)
	} else {
		view.PinnedPosts = pinned
	}

	// Compute follow state for other users' profiles (BUG-3).
	if !data.IsOwnProfile {
		view.IsFollowing = follow.IsFollowing(h.db, r.Context(), currentUserID, profileID)
	} else {
		// Owner-only: connections celebrating a birthday/anniversary this week.
		celebrations, err := h.getWeeklyCelebrations(r, currentUserID)
		if err != nil {
			slog.Error("failed to load celebrations", "error", err)
		} else {
			view.Celebrations = celebrations
		}
	}

	if err := h.pages["profile.html"].ExecuteTemplate(w, "base", view); err != nil {
		slog.Error("failed to render profile edit", "error", err)
	}
}

// showPrintProfile renders a clean, standalone, print-optimized view of a
// profile (no site chrome). It mirrors the resume print page: its own top-level
// template executed directly (not the site "base").
func (h *Handler) showPrintProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("id")
	currentUserID := middleware.GetUserID(r.Context())

	data, err := h.getProfileData(r, profileID, currentUserID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to load profile for print", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.pages["profile_print.html"].ExecuteTemplate(w, "profile_print", data); err != nil {
		slog.Error("failed to render profile print", "error", err)
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

	workStatus := strings.TrimSpace(r.FormValue("work_status"))
	if !IsValidWorkStatus(workStatus) {
		workStatus = "in_office"
	}

	visibility := strings.TrimSpace(r.FormValue("profile_visibility"))
	if !IsValidVisibility(visibility) {
		visibility = "everyone"
	}

	fields := profileFields{
		FirstName:         strings.TrimSpace(r.FormValue("first_name")),
		LastName:          strings.TrimSpace(r.FormValue("last_name")),
		Headline:          strings.TrimSpace(r.FormValue("headline")),
		About:             strings.TrimSpace(r.FormValue("about")),
		Location:          strings.TrimSpace(r.FormValue("location")),
		Birthday:          dateOrNull(r.FormValue("birthday")),
		HireDate:          dateOrNull(r.FormValue("hire_date")),
		WorkStatus:        workStatus,
		WorkStatusUntil:   dateOrNull(r.FormValue("work_status_until")),
		ProfileVisibility: visibility,
	}

	if err := updateProfileFields(r.Context(), h.db, userID, fields); err != nil {
		slog.Error("failed to update profile", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Award badges (e.g. Profile Complete). Non-fatal: never block the update.
	badge.CheckAndAward(r.Context(), h.db, userID)

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

// profileFields holds the editable user fields written by handleEditProfile.
// Birthday and HireDate use sql.NullString so an empty form value persists as
// SQL NULL rather than an empty/zero date.
type profileFields struct {
	FirstName         string
	LastName          string
	Headline          string
	About             string
	Location          string
	Birthday          sql.NullString
	HireDate          sql.NullString
	WorkStatus        string
	WorkStatusUntil   sql.NullString
	ProfileVisibility string
}

// updateProfileFields writes the editable profile fields for userID, including
// the nullable birthday and hire_date DATE columns and the profile_visibility
// privacy setting.
func updateProfileFields(ctx context.Context, db *sql.DB, userID string, f profileFields) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users SET first_name = $1, last_name = $2, headline = $3, about = $4, location = $5,
		 birthday = $6, hire_date = $7, work_status = $8, work_status_until = $9,
		 profile_visibility = $10, updated_at = NOW()
		 WHERE id = $11`,
		f.FirstName, f.LastName, f.Headline, f.About, f.Location, f.Birthday, f.HireDate,
		f.WorkStatus, f.WorkStatusUntil, f.ProfileVisibility, userID,
	)
	return err
}

// dateOrNull turns an HTML date input value into a sql.NullString suitable for
// a nullable DATE column: a blank value becomes SQL NULL, anything else is
// passed through (Postgres parses the YYYY-MM-DD form).
func dateOrNull(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func (h *Handler) handleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}

	contentType := http.DetectContentType(buf[:n])
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}

	if !allowedTypes[contentType] {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
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

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE users SET avatar_data = $1, avatar_content_type = $2, avatar_url = $3, updated_at = NOW()
		 WHERE id = $4`,
		data, contentType, "/avatar", userID,
	)
	if err != nil {
		slog.Error("failed to save avatar", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile/me", http.StatusSeeOther)
}

func (h *Handler) ServeAvatar(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("id")

	var data []byte
	var contentType string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT avatar_data, avatar_content_type FROM users WHERE id = $1`,
		profileID,
	).Scan(&data, &contentType)
	if err != nil || data == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

// pinPost inserts a pin row for (userID, postID) but only if the post belongs
// to userID. Returns false (without error) when the post does not exist or is
// not owned by the user, so callers can return 403/400. The insert is
// idempotent via ON CONFLICT DO NOTHING.
func pinPost(ctx context.Context, db *sql.DB, userID, postID string) (bool, error) {
	var ownerID string
	err := db.QueryRowContext(ctx, `SELECT user_id FROM posts WHERE id = $1`, postID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if ownerID != userID {
		return false, nil
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO pinned_profile_posts (user_id, post_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, userID, postID,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}

// unpinPost removes the pin row for (userID, postID).
func unpinPost(ctx context.Context, db *sql.DB, userID, postID string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM pinned_profile_posts WHERE user_id = $1 AND post_id = $2`,
		userID, postID,
	)
	return err
}

func (h *Handler) handlePinPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	ok, err := pinPost(r.Context(), h.db, userID, postID)
	if err != nil {
		slog.Error("failed to pin post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "You can only pin your own posts", http.StatusForbidden)
		return
	}

	h.redirectBack(w, r, userID)
}

func (h *Handler) handleUnpinPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	if err := unpinPost(r.Context(), h.db, userID, postID); err != nil {
		slog.Error("failed to unpin post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.redirectBack(w, r, userID)
}

// redirectBack returns the user to the referring page, falling back to their
// own profile when no referer is present.
func (h *Handler) redirectBack(w http.ResponseWriter, r *http.Request, userID string) {
	dest := r.Referer()
	if dest == "" {
		dest = "/profile/" + userID
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /profile/edit", requireAuth(http.HandlerFunc(h.showEditProfile)))
	mux.Handle("POST /profile/edit", requireAuth(http.HandlerFunc(h.handleEditProfile)))
	mux.Handle("GET /profile/me", requireAuth(http.HandlerFunc(h.showOwnProfile)))
	mux.Handle("GET /profile/experience/add", requireAuth(http.HandlerFunc(h.showAddExperience)))
	mux.Handle("POST /profile/experience/add", requireAuth(http.HandlerFunc(h.handleAddExperience)))
	mux.Handle("GET /profile/experience/{id}/edit", requireAuth(http.HandlerFunc(h.showEditExperience)))
	mux.Handle("POST /profile/experience/{id}/edit", requireAuth(http.HandlerFunc(h.handleEditExperience)))
	mux.Handle("POST /profile/experience/{id}/delete", requireAuth(http.HandlerFunc(h.handleDeleteExperience)))
	mux.Handle("GET /profile/education/add", requireAuth(http.HandlerFunc(h.showAddEducation)))
	mux.Handle("POST /profile/education/add", requireAuth(http.HandlerFunc(h.handleAddEducation)))
	mux.Handle("GET /profile/education/{id}/edit", requireAuth(http.HandlerFunc(h.showEditEducation)))
	mux.Handle("POST /profile/education/{id}/edit", requireAuth(http.HandlerFunc(h.handleEditEducation)))
	mux.Handle("POST /profile/education/{id}/delete", requireAuth(http.HandlerFunc(h.handleDeleteEducation)))
	mux.Handle("GET /profile/skills/edit", requireAuth(http.HandlerFunc(h.showSkills)))
	mux.Handle("POST /profile/skills/add", requireAuth(http.HandlerFunc(h.handleAddSkill)))
	mux.Handle("POST /profile/skills/{id}/delete", requireAuth(http.HandlerFunc(h.handleDeleteSkill)))
	mux.Handle("POST /profile/skills/{id}/endorse", requireAuth(http.HandlerFunc(h.toggleEndorsement)))
	mux.Handle("POST /profile/posts/{id}/pin", requireAuth(http.HandlerFunc(h.handlePinPost)))
	mux.Handle("POST /profile/posts/{id}/unpin", requireAuth(http.HandlerFunc(h.handleUnpinPost)))
	mux.Handle("GET /profile/{id}/print", requireAuth(http.HandlerFunc(h.showPrintProfile)))
	mux.Handle("GET /profile/{id}", requireAuth(http.HandlerFunc(h.showProfile)))
	mux.Handle("POST /profile/avatar", requireAuth(http.HandlerFunc(h.handleAvatarUpload)))
}

func (h *Handler) toggleEndorsement(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	skillID := r.PathValue("id")

	// Get skill owner to prevent self-endorsement
	var skillOwner string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM skills WHERE id = $1`, skillID,
	).Scan(&skillOwner)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if skillOwner == userID {
		http.Error(w, "Cannot endorse your own skill", http.StatusBadRequest)
		return
	}

	var exists bool
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM skill_endorsements WHERE skill_id = $1 AND endorser_id = $2)`,
		skillID, userID,
	).Scan(&exists)

	if exists {
		h.db.ExecContext(r.Context(),
			`DELETE FROM skill_endorsements WHERE skill_id = $1 AND endorser_id = $2`,
			skillID, userID,
		)
	} else {
		h.db.ExecContext(r.Context(),
			`INSERT INTO skill_endorsements (skill_id, endorser_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			skillID, userID,
		)
	}

	http.Redirect(w, r, "/profile/"+skillOwner, http.StatusSeeOther)
}
