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
		h.pages["register.html"].ExecuteTemplate(w, "base", map[string]string {
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
			h.pages["register.html"].ExecuteTemplate(w, "base", map[string]string {
				"Error": "An account with this email already exists.",
			})
			return
		}
		slog.Error("failed to create user", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
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
		h.pages["login.html"].ExecuteTemplate(w, "base", map[string]string {
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
		h.pages["login.html"].ExecuteTemplate(w, "base", map[string]string {
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
