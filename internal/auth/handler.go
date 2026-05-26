package auth

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db       *sql.DB
	pages    map[string]*template.Template
	sessions *SessionManager
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, sessions *SessionManager) *Handler {
	return &Handler{db: db, pages: pages, sessions: sessions}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /register", h.showRegister)
	mux.HandleFunc("POST /register", h.handleRegister)
	mux.HandleFunc("GET /login", h.showLogin)
	mux.HandleFunc("POST /login", h.handleLogin)
	mux.HandleFunc("POST /logout", h.handleLogout)

	// Active sessions / remote logout. These require auth, but the auth package
	// cannot import internal/middleware (that would create an import cycle, since
	// middleware imports auth). They are guarded by h.requireAuth, a self-contained
	// guard built on the SessionManager. Namespaced under /settings to avoid
	// collisions with existing route patterns.
	mux.Handle("GET /settings/sessions", h.requireAuth(http.HandlerFunc(h.showSessions)))
	mux.Handle("POST /settings/sessions/revoke", h.requireAuth(http.HandlerFunc(h.revokeSession)))
}

// requireAuth is a package-local auth guard. It mirrors middleware.RequireAuth
// but lives in the auth package to avoid an import cycle. It validates the
// session and stashes the userID in the request context under authUserIDKey.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := h.sessions.GetUserID(r.Context(), r)
		if err != nil || userID == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := contextWithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// navInfo holds the navbar fields the shared layout templates read via .Nav.
// It mirrors middleware.UserInfo's field names so templates render identically.
type navInfo struct {
	ID        string
	FirstName string
	LastName  string
	Initials  string
	AvatarURL string
	Role      string
}

// loadNav builds the navbar data for a logged-in user, mirroring the lookup in
// middleware.RequireAuthWithDB. Returns the userID alongside the nav info.
func (h *Handler) loadNav(r *http.Request, userID string) navInfo {
	info := navInfo{ID: userID}
	var avatarURL sql.NullString
	err := h.db.QueryRowContext(r.Context(),
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
	}
	return info
}

// sessionRow is one row in the active-sessions table.
type sessionRow struct {
	// MaskedID is a short, non-sensitive identifier for display.
	MaskedID  string
	Token     string
	UserAgent string
	CreatedAt string
	IsCurrent bool
}

type sessionsPage struct {
	UserID   string
	Nav      navInfo
	Sessions []sessionRow
	Revoked  bool
}

func (h *Handler) showSessions(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	current := h.sessions.CurrentToken(r)

	infos, err := h.sessions.ListSessions(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list sessions", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rows := make([]sessionRow, 0, len(infos))
	for _, info := range infos {
		row := sessionRow{
			Token:     info.Token,
			MaskedID:  maskToken(info.Token),
			UserAgent: info.UserAgent,
			IsCurrent: info.Token == current,
		}
		if row.UserAgent == "" {
			row.UserAgent = "Unknown device"
		}
		if !info.CreatedAt.IsZero() {
			row.CreatedAt = info.CreatedAt.Local().Format("Jan 2, 2006 3:04 PM")
		} else {
			row.CreatedAt = "Unknown"
		}
		rows = append(rows, row)
	}

	data := sessionsPage{
		UserID:   userID,
		Nav:      h.loadNav(r, userID),
		Sessions: rows,
		Revoked:  r.URL.Query().Get("revoked") == "1",
	}

	if err := h.pages["sessions.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render sessions", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("session_id")
	current := h.sessions.CurrentToken(r)

	// Revoking the current session is equivalent to signing out: clear the
	// cookie and redirect to login rather than back to a page they can't view.
	if token == current && token != "" {
		h.sessions.Destroy(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := h.sessions.RevokeSession(r.Context(), userID, token); err != nil {
		slog.Error("failed to revoke session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/sessions?revoked=1", http.StatusSeeOther)
}

// maskToken returns a short, display-safe fragment of a session token.
func maskToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:4] + "…" + token[len(token)-4:]
}

func (h *Handler) showRegister(w http.ResponseWriter, r *http.Request) {
	h.pages["register.html"].ExecuteTemplate(w, "base", nil)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))

	if email == "" || password == "" || firstName == "" || lastName == "" {
		h.pages["register.html"].ExecuteTemplate(w, "base", map[string]string{
			"Error": "All fields required.",
		})
		return
	}

	if len(password) < 8 {
		h.pages["register.html"].ExecuteTemplate(w, "base", map[string]string{
			"Error": "Password must be at least 8 characters.",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var userID string
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO users (email, password_hash, first_name, last_name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		email, string(hash), firstName, lastName,
	).Scan(&userID)

	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			h.pages["register.html"].ExecuteTemplate(w, "base", map[string]string{
				"Error": "An account with this email already exists.",
			})
			return
		}
		slog.Error("failed to create user", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// New-hire announcement: auto-post an intro from the new employee. It surfaces
	// in the feed as they make connections. Best-effort — never block signup.
	if _, err := h.db.ExecContext(r.Context(),
		`INSERT INTO posts (user_id, content) VALUES ($1, $2)`,
		userID, firstName+" "+lastName+" just joined USDA JobPortal. Welcome aboard! 👋 #newhire",
	); err != nil {
		slog.Error("failed to create new-hire announcement", "error", err)
	}

	if err := h.sessions.Create(w, r, userID); err != nil {
		slog.Error("failed to create session", "error", err)
	}

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	err := h.pages["login.html"].ExecuteTemplate(w, "base", nil)
	if err != nil {
		slog.Error("failed to render login", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	var userID, hash string
	err := h.db.QueryRowContext(r.Context(),
		"SELECT id, password_hash FROM users where email = $1", email,
	).Scan(&userID, &hash)

	if err == sql.ErrNoRows {
		h.pages["login.html"].ExecuteTemplate(w, "base", map[string]string{
			"Error": "Invalid email or password.",
		})
		return
	}

	if err != nil {
		slog.Error("failed to query user", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		h.pages["login.html"].ExecuteTemplate(w, "base", map[string]string{
			"Error": "Invalid email or password.",
		})
		return
	}

	if err := h.sessions.Create(w, r, userID); err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Destroy(w, r)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
