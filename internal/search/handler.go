package search

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
	conn *connection.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, conn *connection.Handler) *Handler {
	return &Handler { db: db, pages: pages, conn: conn }
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /search", requireAuth(http.HandlerFunc(h.handleSearch)))
}

type SearchResult struct {
	ID        		 string
	FirstName 		 string
	LastName  		 string
	Headline  		 string
	AvatarURL 		 string
	Location  		 string
	ConnectionStatus string
}

type SearchPage struct {
	UserID   string
	Query    string
	Results  []SearchResult
	Searched bool
	ConnectionStatus string
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	data := SearchPage {
		UserID:   userID,
		Query:    query,
		Searched: query != "",
		ConnectionStatus: "none",
	}

	if query != "" {
		results, err := h.searchUsers(r, query, userID)
		if err != nil {
			slog.Error("search failed", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		data.Results = results
	}

	if len(data.Results) > 0 {
		for _, result := range data.Results {
			status, err := h.conn.GetConnectionStatus(userID, result.ID)
			if err != nil {
				slog.Error("failed to get connection status during search", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		 	result.ConnectionStatus = status
		}
	}

	if err := h.pages["search.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render search", "error", err)
	}
}

func (h *Handler) searchUsers(r *http.Request, query string, currentUserID string) ([]SearchResult, error) {
	searchTerm := "%" + strings.ToLower(query) + "%"

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, first_name, last_name, headline, avatar_url, location
		 FROM users
		 WHERE id != $1
		 	AND (
		 		lower(first_name || ' ' || last_name) LIKE $2
		 		OR lower(headline) LIKE $2
		 		OR lower(location) LIKE $2
		 	)
		 ORDER BY
		 	CASE WHEN lower(first_name || ' ' || last_name) LIKE $2 THEN 0 ELSE 1 END,
		 	first_name, last_name
		 LIMIT 20`,
		 currentUserID, searchTerm,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.ID, &result.FirstName, &result.LastName, &result.Headline, &result.AvatarURL, &result.Location); err != nil {
			return nil, err
		}

		results = append(results, result)
	}
	return results, rows.Err()
}
