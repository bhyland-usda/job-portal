package middleware

import (
	"database/sql"
	"net/http"
	"strings"
)

// RequireAUP returns middleware that gates authenticated users behind
// acceptance of the Acceptable Use Policy. If the current user's
// users.aup_accepted_at IS NULL, the request is redirected to /aup.
//
// To avoid a redirect loop (and to keep the AUP pages, logout, and static
// assets reachable), the following paths are always allowed through without
// a DB lookup or redirect:
//
//	/aup, /aup/accept, /logout, and any /static/* path.
//
// Requests without an authenticated user in the context are passed straight
// through; authentication is enforced separately by the auth middleware.
func RequireAUP(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAUPExempt(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			userID := GetUserID(r.Context())
			if userID == "" {
				// Not authenticated here; let the auth middleware decide.
				next.ServeHTTP(w, r)
				return
			}

			var acceptedAt sql.NullTime
			err := db.QueryRowContext(r.Context(),
				`SELECT aup_accepted_at FROM users WHERE id = $1`,
				userID,
			).Scan(&acceptedAt)
			if err != nil || !acceptedAt.Valid {
				http.Redirect(w, r, "/aup", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAUPExempt reports whether a path must bypass the AUP gate.
func isAUPExempt(path string) bool {
	switch path {
	case "/aup", "/aup/accept", "/logout":
		return true
	}
	return strings.HasPrefix(path, "/static/")
}
