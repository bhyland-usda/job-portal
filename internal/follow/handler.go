package follow

import (
	"database/sql"
	"log/slog"
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
	mux.Handle("POST /follow/{userID}", requireAuth(http.HandlerFunc(h.toggleFollow)))
}

func (h *Handler) toggleFollow(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	targetID := r.PathValue("userID")

	if userID == targetID {
		http.Error(w, "Cannot follow yourself", http.StatusBadRequest)
		return
	}

	var exists bool
	h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followed_id = $2)`,
		userID, targetID,
	).Scan(&exists)

	if exists {
		_, err := h.db.ExecContext(r.Context(),
			`DELETE FROM follows WHERE follower_id = $1 AND followed_id = $2`,
			userID, targetID,
		)
		if err != nil {
			slog.Error("failed to unfollow", "error", err)
		}
	} else {
		_, err := h.db.ExecContext(r.Context(),
			`INSERT INTO follows (follower_id, followed_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			userID, targetID,
		)
		if err != nil {
			slog.Error("failed to follow", "error", err)
		}
	}

	http.Redirect(w, r, "/profile/"+targetID, http.StatusSeeOther)
}

func IsFollowing(db *sql.DB, ctx interface{ Value(interface{}) interface{} }, followerID, followedID string) bool {
	var exists bool
	db.QueryRow(`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followed_id = $2)`,
		followerID, followedID,
	).Scan(&exists)
	return exists
}
