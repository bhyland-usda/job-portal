package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UserInfoKey contextKey = "userInfo"

type UserInfo struct {
	ID        string
	FirstName string
	LastName  string
	Initials  string
	AvatarURL string
	Role      string
}

func RequireAuthWithDB(sessions *auth.SessionManager, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := sessions.GetUserID(r.Context(), r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			var info UserInfo
			var avatarURL sql.NullString
			err = db.QueryRowContext(r.Context(),
				`SELECT id, first_name, last_name, role, avatar_url FROM users WHERE id = $1`,
				userID,
			).Scan(&info.ID, &info.FirstName, &info.LastName, &info.Role, &avatarURL)
			if err == nil {
				if len(info.FirstName) > 0 && len(info.LastName) > 0 {
					info.Initials = string(info.FirstName[0]) + string(info.LastName[0])
				}
				if avatarURL.Valid && avatarURL.String != "" {
					info.AvatarURL = "/avatar/" + userID
				}
				ctx = context.WithValue(ctx, UserInfoKey, info)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAuth(sessions *auth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := sessions.GetUserID(r.Context(), r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserInfo(ctx context.Context) UserInfo {
	info, _ := ctx.Value(UserInfoKey).(UserInfo)
	return info
}

func RequireRole(db *sql.DB, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			var role string
			err := db.QueryRowContext(r.Context(),
				`SELECT role FROM users WHERE id = $1`,
				userID,
			).Scan(&role)

			if err != nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

func GetUserRole(db *sql.DB, ctx context.Context, userID string) string {
	var role string
	err := db.QueryRowContext(ctx,
		`SELECT role FROM users WHERE id = $1`,
		userID,
	).Scan(&role)
	if err != nil {
		return "employee"
	}

	return role
}

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDKey).(string)
	return userID
}
