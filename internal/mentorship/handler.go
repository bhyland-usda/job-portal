package mentorship

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
)

type Mentorship struct {
	ID         string
	MentorID   string
	MentorName string
	MenteeID   string
	MenteeName string
	Status     string
	CreatedAt  time.Time
}

type Mentor struct {
	ID           string
	FirstName    string
	LastName     string
	Headline     string
	SkillCount   int
	SharedSkills int
}

type MentorPage struct {
	middleware.BaseData
	Tab              string
	Mentorships      []Mentorship
	AvailableMentors []Mentor
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	notif *notification.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, notif *notification.Handler) *Handler {
	return &Handler{db: db, pages: pages, notif: notif}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /mentorship", requireAuth(http.HandlerFunc(h.showMentorship)))
	mux.Handle("POST /mentors/request/{userID}", requireAuth(http.HandlerFunc(h.requestMentorship)))
	mux.Handle("POST /mentorship/{id}/accept", requireAuth(http.HandlerFunc(h.acceptMentorship)))
	mux.Handle("POST /mentorship/{id}/decline", requireAuth(http.HandlerFunc(h.declineMentorship)))
	mux.Handle("POST /mentorship/{id}/complete", requireAuth(http.HandlerFunc(h.completeMentorship)))
}

func (h *Handler) showMentorship(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "active"
	}

	// Load mentorships (pending + active) where user is mentor or mentee
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT m.id, m.mentor_id,
			CONCAT(mentor.first_name, ' ', mentor.last_name),
			m.mentee_id,
			CONCAT(mentee.first_name, ' ', mentee.last_name),
			m.status, m.created_at
		 FROM mentorships m
		 JOIN users mentor ON mentor.id = m.mentor_id
		 JOIN users mentee ON mentee.id = m.mentee_id
		 WHERE (m.mentor_id = $1 OR m.mentee_id = $1)
		   AND m.status IN ('pending', 'active')
		 ORDER BY m.created_at DESC`, userID,
	)
	if err != nil {
		slog.Error("failed to load mentorships", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var mentorships []Mentorship
	for rows.Next() {
		var m Mentorship
		if err := rows.Scan(&m.ID, &m.MentorID, &m.MentorName,
			&m.MenteeID, &m.MenteeName, &m.Status, &m.CreatedAt); err != nil {
			slog.Error("failed to scan mentorship", "error", err)
			continue
		}
		mentorships = append(mentorships, m)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate mentorships", "error", err)
	}

	// Load available mentors (users with more experience entries, ordered by shared skills)
	var mentors []Mentor
	if tab == "find" {
		mentorRows, err := h.db.QueryContext(r.Context(),
			`SELECT u.id, u.first_name, u.last_name, COALESCE(u.headline, ''),
				COUNT(DISTINCT s.id) AS skill_count,
				COUNT(DISTINCT CASE WHEN s.name IN (
					SELECT name FROM skills WHERE user_id = $1
				) THEN s.id END) AS shared_skills
			 FROM users u
			 LEFT JOIN skills s ON s.user_id = u.id
			 WHERE u.id != $1
			   AND (SELECT COUNT(*) FROM experiences WHERE user_id = u.id)
			       > (SELECT COUNT(*) FROM experiences WHERE user_id = $1)
			   AND u.id NOT IN (
			       SELECT mentor_id FROM mentorships
			       WHERE mentee_id = $1 AND status IN ('pending', 'active')
			   )
			 GROUP BY u.id, u.first_name, u.last_name, u.headline
			 ORDER BY shared_skills DESC, skill_count DESC
			 LIMIT 20`, userID,
		)
		if err != nil {
			slog.Error("failed to load available mentors", "error", err)
		} else {
			defer mentorRows.Close()
			for mentorRows.Next() {
				var m Mentor
				if err := mentorRows.Scan(&m.ID, &m.FirstName, &m.LastName,
					&m.Headline, &m.SkillCount, &m.SharedSkills); err != nil {
					slog.Error("failed to scan mentor", "error", err)
					continue
				}
				mentors = append(mentors, m)
			}
			if err := mentorRows.Err(); err != nil {
				slog.Error("failed to iterate mentors", "error", err)
			}
		}
	}

	data := MentorPage{
		BaseData:         middleware.NewBaseData(r),
		Tab:              tab,
		Mentorships:      mentorships,
		AvailableMentors: mentors,
	}

	if err := h.pages["mentorship.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render mentorship", "error", err)
	}
}

func (h *Handler) requestMentorship(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	mentorID := r.PathValue("userID")

	if userID == mentorID {
		http.Error(w, "Cannot mentor yourself", http.StatusBadRequest)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO mentorships (mentor_id, mentee_id, status)
		 VALUES ($1, $2, 'pending')
		 ON CONFLICT (mentor_id, mentee_id) DO NOTHING`,
		mentorID, userID,
	)
	if err != nil {
		slog.Error("failed to request mentorship", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Notify mentor
	var firstName, lastName string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&firstName, &lastName)

	h.notif.CreateNotification(r.Context(), mentorID, userID,
		"mentorship", firstName+" "+lastName+" requested you as a mentor")

	http.Redirect(w, r, "/mentorship?tab=active", http.StatusSeeOther)
}

func (h *Handler) acceptMentorship(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	mentorshipID := r.PathValue("id")

	var menteeID string
	err := h.db.QueryRowContext(r.Context(),
		`UPDATE mentorships SET status = 'active', updated_at = NOW()
		 WHERE id = $1 AND mentor_id = $2 AND status = 'pending'
		 RETURNING mentee_id`,
		mentorshipID, userID,
	).Scan(&menteeID)
	if err != nil {
		slog.Error("failed to accept mentorship", "error", err)
		http.Error(w, "No pending request found", http.StatusBadRequest)
		return
	}

	// Notify mentee
	var firstName, lastName string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&firstName, &lastName)

	h.notif.CreateNotification(r.Context(), menteeID, userID,
		"mentorship", firstName+" "+lastName+" accepted your mentorship request")

	// Accepting a request makes this user a mentor — check the Mentor badge.
	badge.CheckAndAward(r.Context(), h.db, userID)

	http.Redirect(w, r, "/mentorship", http.StatusSeeOther)
}

func (h *Handler) declineMentorship(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	mentorshipID := r.PathValue("id")

	var menteeID string
	err := h.db.QueryRowContext(r.Context(),
		`DELETE FROM mentorships
		 WHERE id = $1 AND mentor_id = $2 AND status = 'pending'
		 RETURNING mentee_id`,
		mentorshipID, userID,
	).Scan(&menteeID)
	if err != nil {
		slog.Error("failed to decline mentorship", "error", err)
		http.Error(w, "No pending request found", http.StatusBadRequest)
		return
	}

	// Notify mentee
	var firstName, lastName string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&firstName, &lastName)

	h.notif.CreateNotification(r.Context(), menteeID, userID,
		"mentorship", firstName+" "+lastName+" declined your mentorship request")

	http.Redirect(w, r, "/mentorship", http.StatusSeeOther)
}

func (h *Handler) completeMentorship(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	mentorshipID := r.PathValue("id")

	var mentorID, menteeID string
	err := h.db.QueryRowContext(r.Context(),
		`UPDATE mentorships SET status = 'completed', updated_at = NOW()
		 WHERE id = $1 AND (mentor_id = $2 OR mentee_id = $2) AND status = 'active'
		 RETURNING mentor_id, mentee_id`,
		mentorshipID, userID,
	).Scan(&mentorID, &menteeID)
	if err != nil {
		slog.Error("failed to complete mentorship", "error", err)
		http.Error(w, "No active mentorship found", http.StatusBadRequest)
		return
	}

	// Notify the other party
	otherID := mentorID
	if userID == mentorID {
		otherID = menteeID
	}

	var firstName, lastName string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&firstName, &lastName)

	h.notif.CreateNotification(r.Context(), otherID, userID,
		"mentorship", firstName+" "+lastName+" marked your mentorship as complete")

	http.Redirect(w, r, "/mentorship", http.StatusSeeOther)
}
