package search

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bhyland-usda/job-portal/internal/connection"
	"github.com/bhyland-usda/job-portal/internal/middleware"
)

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	conn  *connection.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, conn *connection.Handler) *Handler {
	return &Handler{db: db, pages: pages, conn: conn}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /search", requireAuth(http.HandlerFunc(h.handleSearch)))
}

type SearchResult struct {
	ID               string
	FirstName        string
	LastName         string
	Headline         string
	AvatarURL        string
	Location         string
	ConnectionStatus string
}

type SearchPage struct {
	middleware.BaseData
	Query            string
	Results          []SearchResult
	Searched         bool
	ConnectionStatus string
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	viewerRole := middleware.GetUserInfo(r.Context()).Role
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	data := SearchPage{
		BaseData:         middleware.NewBaseData(r),
		Query:            query,
		Searched:         query != "",
		ConnectionStatus: "none",
	}

	if query != "" {
		results, err := h.searchUsers(r, query, userID, viewerRole)
		if err != nil {
			slog.Error("search failed", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		data.Results = results
	}

	if len(data.Results) > 0 {
		for i := range data.Results {
			status, err := h.conn.GetConnectionStatus(userID, data.Results[i].ID)
			if err != nil {
				slog.Error("failed to get connection status during search", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			data.Results[i].ConnectionStatus = status
		}
	}

	if err := h.pages["search.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render search", "error", err)
	}
}

func (h *Handler) searchUsers(r *http.Request, query string, currentUserID string, viewerRole string) ([]SearchResult, error) {
	searchTerm := "%" + strings.ToLower(query) + "%"

	// Privacy: exclude users who have set their profile to 'private' from search
	// results for everyone except admins (who may see all profiles). The owner is
	// already excluded by the id != $1 clause.
	visibilityFilter := ""
	if viewerRole != "admin" {
		visibilityFilter = "AND profile_visibility <> 'private'"
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, first_name, last_name, headline, avatar_url, location
		 FROM users
		 WHERE id != $1
		 	`+visibilityFilter+`
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
		result.AvatarURL = middleware.NormalizeAvatarURL(result.ID, result.AvatarURL)

		results = append(results, result)
	}
	return results, rows.Err()
}
