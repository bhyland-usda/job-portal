package verification

import (
	"database/sql"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("POST /profile/skills/{id}/verify", requireAuth(http.HandlerFunc(h.toggleVerification)))
}

func (h *Handler) toggleVerification(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	skillID := r.PathValue("id")

	var skillOwner string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM skills WHERE id = $1`, skillID,
	).Scan(&skillOwner)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if skillOwner == userID {
		http.Error(w, "Cannot verify your own skill", http.StatusBadRequest)
		return
	}

	var exists bool
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM skill_verifications WHERE skill_id = $1 AND verifier_id = $2)`,
		skillID, userID,
	).Scan(&exists)

	if exists {
		h.db.ExecContext(r.Context(),
			`DELETE FROM skill_verifications WHERE skill_id = $1 AND verifier_id = $2`,
			skillID, userID,
		)
	} else {
		h.db.ExecContext(r.Context(),
			`INSERT INTO skill_verifications (skill_id, verifier_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			skillID, userID,
		)
	}

	http.Redirect(w, r, "/profile/"+skillOwner, http.StatusSeeOther)
}

func GetVerificationCount(db *sql.DB, skillID string) int {
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM skill_verifications WHERE skill_id = $1`, skillID).Scan(&count)
	return count
}
