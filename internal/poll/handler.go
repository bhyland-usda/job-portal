package poll

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type PollOption struct {
	ID        string
	Label     string
	SortOrder int
	VoteCount int
	Percent   int
}

type Poll struct {
	ID                string
	AuthorID          string
	AuthorName        string
	Question          string
	ClosesAt          *time.Time
	CreatedAt         time.Time
	Options           []PollOption
	TotalVotes        int
	UserVotedOptionID string
}

type PollPage struct {
	middleware.BaseData
	Polls []Poll
}

type CreatePollPage struct {
	middleware.BaseData
	Error string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireManager func(http.Handler) http.Handler) {
	mux.Handle("GET /polls", requireAuth(http.HandlerFunc(h.showPolls)))
	mux.Handle("GET /polls/create", requireAuth(requireManager(http.HandlerFunc(h.showCreate))))
	mux.Handle("POST /polls/create", requireAuth(requireManager(http.HandlerFunc(h.handleCreate))))
	mux.Handle("POST /polls/{id}/vote", requireAuth(http.HandlerFunc(h.handleVote)))
}

func (h *Handler) showPolls(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT p.id, p.author_id, u.first_name || ' ' || u.last_name,
		        p.question, p.closes_at, p.created_at
		 FROM polls p
		 JOIN users u ON u.id = p.author_id
		 WHERE p.closes_at IS NULL OR p.closes_at > NOW()
		 ORDER BY p.created_at DESC`)
	if err != nil {
		slog.Error("failed to load polls", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var polls []Poll
	for rows.Next() {
		var p Poll
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName,
			&p.Question, &p.ClosesAt, &p.CreatedAt); err != nil {
			slog.Error("failed to scan poll", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		polls = append(polls, p)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to iterate polls", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load options, vote counts, and user's vote for each poll
	for i := range polls {
		optRows, err := h.db.QueryContext(r.Context(),
			`SELECT po.id, po.label, po.sort_order,
			        COUNT(pv.user_id) AS vote_count
			 FROM poll_options po
			 LEFT JOIN poll_votes pv ON pv.option_id = po.id
			 WHERE po.poll_id = $1
			 GROUP BY po.id, po.label, po.sort_order
			 ORDER BY po.sort_order`, polls[i].ID)
		if err != nil {
			slog.Error("failed to load poll options", "error", err)
			continue
		}

		var options []PollOption
		total := 0
		for optRows.Next() {
			var o PollOption
			if err := optRows.Scan(&o.ID, &o.Label, &o.SortOrder, &o.VoteCount); err != nil {
				slog.Error("failed to scan poll option", "error", err)
				break
			}
			total += o.VoteCount
			options = append(options, o)
		}
		optRows.Close()

		if total > 0 {
			for j := range options {
				options[j].Percent = options[j].VoteCount * 100 / total
			}
		}
		polls[i].Options = options
		polls[i].TotalVotes = total

		// Check if user voted
		var votedOptionID sql.NullString
		err = h.db.QueryRowContext(r.Context(),
			`SELECT option_id FROM poll_votes WHERE poll_id = $1 AND user_id = $2`,
			polls[i].ID, userID).Scan(&votedOptionID)
		if err == nil && votedOptionID.Valid {
			polls[i].UserVotedOptionID = votedOptionID.String
		}
	}

	data := PollPage{
		BaseData: middleware.NewBaseData(r),
		Polls:    polls,
	}

	if err := h.pages["polls.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render polls", "error", err)
	}
}

func (h *Handler) showCreate(w http.ResponseWriter, r *http.Request) {
	data := CreatePollPage{BaseData: middleware.NewBaseData(r)}
	if err := h.pages["poll_create.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render poll create", "error", err)
	}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(r.FormValue("question"))
	if question == "" {
		h.renderCreateError(w, r, "Question is required.")
		return
	}

	var options []string
	for i := 1; i <= 5; i++ {
		opt := strings.TrimSpace(r.FormValue("option" + string(rune('0'+i))))
		if opt != "" {
			options = append(options, opt)
		}
	}
	if len(options) < 2 {
		h.renderCreateError(w, r, "At least 2 options are required.")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin tx", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var pollID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO polls (author_id, question) VALUES ($1, $2) RETURNING id`,
		userID, question).Scan(&pollID)
	if err != nil {
		slog.Error("failed to insert poll", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for i, opt := range options {
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO poll_options (poll_id, label, sort_order) VALUES ($1, $2, $3)`,
			pollID, opt, i)
		if err != nil {
			slog.Error("failed to insert poll option", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit poll", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/polls", http.StatusSeeOther)
}

func (h *Handler) handleVote(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pollID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	optionID := r.FormValue("option")
	if optionID == "" {
		http.Redirect(w, r, "/polls", http.StatusSeeOther)
		return
	}

	// Check poll isn't closed
	var closesAt sql.NullTime
	err := h.db.QueryRowContext(r.Context(),
		`SELECT closes_at FROM polls WHERE id = $1`, pollID).Scan(&closesAt)
	if err != nil {
		slog.Error("failed to check poll", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if closesAt.Valid && closesAt.Time.Before(time.Now()) {
		http.Redirect(w, r, "/polls", http.StatusSeeOther)
		return
	}

	// Check user hasn't already voted
	var exists bool
	err = h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM poll_votes WHERE poll_id = $1 AND user_id = $2)`,
		pollID, userID).Scan(&exists)
	if err != nil {
		slog.Error("failed to check vote", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Redirect(w, r, "/polls", http.StatusSeeOther)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES ($1, $2, $3)`,
		pollID, userID, optionID)
	if err != nil {
		slog.Error("failed to insert vote", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/polls", http.StatusSeeOther)
}

func (h *Handler) renderCreateError(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := CreatePollPage{BaseData: middleware.NewBaseData(r), Error: errMsg}
	if err := h.pages["poll_create.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render poll create error", "error", err)
	}
}
