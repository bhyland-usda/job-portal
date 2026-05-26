// Package insights provides workforce visualization pages: a department x
// skill heat map for managers, and a personal connection-network graph for
// every authenticated user.
package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// Handler serves the insights pages. It mirrors internal/news: a *sql.DB plus
// the shared map of parsed page templates.
type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
}

// NewHandler constructs an insights Handler.
func NewHandler(db *sql.DB, pages map[string]*template.Template) *Handler {
	return &Handler{db: db, pages: pages}
}

// RegisterRoutes wires the insights routes. The heat map is gated to managers
// (and admins), the network graph is available to any authenticated user.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth, requireManager func(http.Handler) http.Handler) {
	mux.Handle("GET /insights/heatmap", requireAuth(requireManager(http.HandlerFunc(h.showHeatmap))))
	mux.Handle("GET /insights/network", requireAuth(http.HandlerFunc(h.showNetwork)))
}

// --- Feature 1: Skills Heat Map ---

// HeatCell is a single department x skill intersection. Color is a precomputed
// CSS background value whose intensity scales with Count relative to the grid
// maximum, so the template can apply it as an inline style directly.
type HeatCell struct {
	Count int
	Color template.CSS
}

// shadeColor returns a CSS rgb() value that grows darker/greener as count
// approaches max. A zero count yields white (effectively blank).
func shadeColor(count, max int) template.CSS {
	if count <= 0 || max <= 0 {
		return template.CSS("#ffffff")
	}
	// Interpolate from white (255,255,255) toward USDA green (46,125,50).
	intensity := float64(count) / float64(max)
	r := int(255 + intensity*(46-255))
	g := int(255 + intensity*(125-255))
	b := int(255 + intensity*(50-255))
	return template.CSS(fmt.Sprintf("rgb(%d,%d,%d)", r, g, b))
}

// HeatRow is one department's row of cells, aligned to HeatmapPage.Skills.
type HeatRow struct {
	Department string
	Cells      []HeatCell
}

// HeatmapPage is the data handed to templates/insights/heatmap.html.
type HeatmapPage struct {
	middleware.BaseData
	Departments []string
	Skills      []string
	Rows        []HeatRow
	MaxCount    int
}

// topSkillsLimit caps the number of skill columns so the grid stays readable.
const topSkillsLimit = 12

// buildHeatmap loads the department x skill employee counts and arranges them
// into an aligned grid. Departments become rows, the most common skills become
// columns, and each cell holds the number of employees in that department who
// list that skill.
func buildHeatmap(ctx context.Context, db *sql.DB) (HeatmapPage, error) {
	// Count distinct employees per (department, skill). A user listing the same
	// skill name twice still counts once thanks to COUNT(DISTINCT u.id). Only
	// the most-common skills (by total employees holding them) are returned as
	// columns to keep the grid manageable.
	const q = `
		SELECT d.name AS department, s.name AS skill, COUNT(DISTINCT u.id) AS cnt
		FROM skills s
		JOIN users u ON u.id = s.user_id
		JOIN departments d ON d.id = u.department_id
		WHERE s.name IN (
			SELECT s2.name
			FROM skills s2
			JOIN users u2 ON u2.id = s2.user_id
			WHERE u2.department_id IS NOT NULL
			GROUP BY s2.name
			ORDER BY COUNT(DISTINCT u2.id) DESC, s2.name ASC
			LIMIT $1
		)
		GROUP BY d.name, s.name
		ORDER BY d.name ASC, s.name ASC`

	rows, err := db.QueryContext(ctx, q, topSkillsLimit)
	if err != nil {
		return HeatmapPage{}, err
	}
	defer rows.Close()

	type key struct {
		dept  string
		skill string
	}
	counts := make(map[key]int)
	deptOrder := []string{}
	skillOrder := []string{}
	deptSeen := map[string]bool{}
	skillSeen := map[string]bool{}
	maxCount := 0

	for rows.Next() {
		var dept, skill string
		var cnt int
		if err := rows.Scan(&dept, &skill, &cnt); err != nil {
			slog.Error("failed to scan heatmap row", "error", err)
			continue
		}
		counts[key{dept, skill}] = cnt
		if !deptSeen[dept] {
			deptSeen[dept] = true
			deptOrder = append(deptOrder, dept)
		}
		if !skillSeen[skill] {
			skillSeen[skill] = true
			skillOrder = append(skillOrder, skill)
		}
		if cnt > maxCount {
			maxCount = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return HeatmapPage{}, err
	}

	page := HeatmapPage{
		Departments: deptOrder,
		Skills:      skillOrder,
		MaxCount:    maxCount,
	}
	for _, d := range deptOrder {
		row := HeatRow{Department: d, Cells: make([]HeatCell, len(skillOrder))}
		for i, s := range skillOrder {
			c := counts[key{d, s}]
			row.Cells[i] = HeatCell{Count: c, Color: shadeColor(c, maxCount)}
		}
		page.Rows = append(page.Rows, row)
	}

	return page, nil
}

func (h *Handler) showHeatmap(w http.ResponseWriter, r *http.Request) {
	page, err := buildHeatmap(r.Context(), h.db)
	if err != nil {
		slog.Error("failed to build skills heatmap", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	page.BaseData = middleware.NewBaseData(r)

	if err := h.pages["heatmap.html"].ExecuteTemplate(w, "base", page); err != nil {
		slog.Error("failed to render heatmap", "error", err)
	}
}

// --- Feature 2: Network Visualization ---

// graphNode is one node in the connection graph, serialized to JSON for the
// front-end SVG layout.
type graphNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Center bool   `json:"center"`
}

// graphEdge connects the center user to one of their accepted connections.
type graphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type graphData struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// NetworkPage is the data handed to templates/insights/network.html. GraphJSON
// is the marshaled graph (already HTML/JS-safe via template.JS in the template)
// consumed by the embedded vanilla-JS layout code.
type NetworkPage struct {
	middleware.BaseData
	GraphJSON       template.JS
	ConnectionCount int
}

// buildNetwork assembles the current user's accepted-connection graph: the user
// at the center, each accepted connection as a node, and an edge from the center
// to each connection.
func buildNetwork(ctx context.Context, db *sql.DB, userID string) (NetworkPage, error) {
	// Center node label.
	var selfName string
	err := db.QueryRowContext(ctx,
		`SELECT first_name || ' ' || last_name FROM users WHERE id = $1`,
		userID,
	).Scan(&selfName)
	if err != nil {
		return NetworkPage{}, err
	}

	graph := graphData{
		Nodes: []graphNode{{ID: userID, Label: selfName, Center: true}},
		Edges: []graphEdge{},
	}

	// Accepted connections in either direction. The CASE picks the "other"
	// party relative to the current user.
	const q = `
		SELECT
			CASE WHEN c.requester_id = $1 THEN c.addressee_id ELSE c.requester_id END AS other_id,
			u.first_name || ' ' || u.last_name AS name
		FROM connections c
		JOIN users u
			ON u.id = CASE WHEN c.requester_id = $1 THEN c.addressee_id ELSE c.requester_id END
		WHERE c.status = 'accepted'
			AND (c.requester_id = $1 OR c.addressee_id = $1)
		ORDER BY name ASC`

	rows, err := db.QueryContext(ctx, q, userID)
	if err != nil {
		return NetworkPage{}, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			slog.Error("failed to scan connection", "error", err)
			continue
		}
		graph.Nodes = append(graph.Nodes, graphNode{ID: id, Label: name})
		graph.Edges = append(graph.Edges, graphEdge{Source: userID, Target: id})
		count++
	}
	if err := rows.Err(); err != nil {
		return NetworkPage{}, err
	}

	raw, err := json.Marshal(graph)
	if err != nil {
		return NetworkPage{}, err
	}

	return NetworkPage{
		GraphJSON:       template.JS(raw),
		ConnectionCount: count,
	}, nil
}

func (h *Handler) showNetwork(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	page, err := buildNetwork(r.Context(), h.db, userID)
	if err != nil {
		slog.Error("failed to build network graph", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	page.BaseData = middleware.NewBaseData(r)

	if err := h.pages["network.html"].ExecuteTemplate(w, "base", page); err != nil {
		slog.Error("failed to render network", "error", err)
	}
}
