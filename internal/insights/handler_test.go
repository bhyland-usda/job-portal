package insights

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// parsePage parses a page template the same way cmd/server/main.go does:
// the shared layouts plus the page file, relative to the repo templates dir.
func parsePage(t *testing.T, name string) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFS(
		os.DirFS("../../templates"),
		"layouts/*.html",
		name,
	)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", name, err)
	}
	return tmpl
}

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	return db, mock
}

// --- Feature 1: Skills Heat Map ---

func TestBuildHeatmap_BuildsGrid(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	// The handler joins skills -> users -> departments and groups by
	// (department, skill) returning a count of employees.
	rows := sqlmock.NewRows([]string{"department", "skill", "cnt"}).
		AddRow("NRCS", "GIS", 5).
		AddRow("NRCS", "Soil Science", 2).
		AddRow("Forest Service", "GIS", 3)

	mock.ExpectQuery("FROM skills").WillReturnRows(rows)

	page, err := buildHeatmap(context.Background(), db)
	if err != nil {
		t.Fatalf("buildHeatmap returned error: %v", err)
	}

	if len(page.Departments) != 2 {
		t.Fatalf("expected 2 departments, got %d: %v", len(page.Departments), page.Departments)
	}
	if len(page.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(page.Skills), page.Skills)
	}

	// Verify a known cell value: NRCS x GIS == 5.
	got := lookupCell(page, "NRCS", "GIS")
	if got != 5 {
		t.Errorf("expected NRCS/GIS count 5, got %d", got)
	}
	// Verify an absent pair yields a zero cell (Forest Service has no Soil Science).
	got = lookupCell(page, "Forest Service", "Soil Science")
	if got != 0 {
		t.Errorf("expected Forest Service/Soil Science count 0, got %d", got)
	}
	// Verify the present cross pair.
	if c := lookupCell(page, "Forest Service", "GIS"); c != 3 {
		t.Errorf("expected Forest Service/GIS count 3, got %d", c)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildHeatmap_MaxCountForShading(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"department", "skill", "cnt"}).
		AddRow("A", "X", 1).
		AddRow("B", "Y", 9)
	mock.ExpectQuery("FROM skills").WillReturnRows(rows)

	page, err := buildHeatmap(context.Background(), db)
	if err != nil {
		t.Fatalf("buildHeatmap returned error: %v", err)
	}
	if page.MaxCount != 9 {
		t.Errorf("expected MaxCount 9, got %d", page.MaxCount)
	}
}

func TestBuildHeatmap_QueryError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectQuery("FROM skills").WillReturnError(sql.ErrConnDone)

	if _, err := buildHeatmap(context.Background(), db); err == nil {
		t.Error("expected error from buildHeatmap, got nil")
	}
}

// lookupCell finds the count for a department/skill pair in the heatmap grid.
func lookupCell(page HeatmapPage, dept, skill string) int {
	di, si := -1, -1
	for i, d := range page.Departments {
		if d == dept {
			di = i
		}
	}
	for i, s := range page.Skills {
		if s == skill {
			si = i
		}
	}
	if di < 0 || si < 0 {
		return -1
	}
	return page.Rows[di].Cells[si].Count
}

// --- Feature 2: Network Visualization ---

func TestBuildNetwork_NodesAndEdges(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	const me = "user-self"

	// Center node lookup.
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id =").
		WithArgs(me).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Self User"))

	// Accepted connections of the current user (either direction).
	connRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow("user-a", "Alice Adams").
		AddRow("user-b", "Bob Brown")
	mock.ExpectQuery("FROM connections").
		WithArgs(me).
		WillReturnRows(connRows)

	page, err := buildNetwork(context.Background(), db, me)
	if err != nil {
		t.Fatalf("buildNetwork returned error: %v", err)
	}

	// Decode the JSON the template will hand to the SVG/JS.
	var graph struct {
		Nodes []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Center bool   `json:"center"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(page.GraphJSON), &graph); err != nil {
		t.Fatalf("GraphJSON is not valid JSON: %v\n%s", err, page.GraphJSON)
	}

	// 1 center + 2 connections == 3 nodes.
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	// 2 edges from center to each connection.
	if len(graph.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph.Edges))
	}

	// Exactly one center node, and it is the current user.
	centerCount := 0
	for _, n := range graph.Nodes {
		if n.Center {
			centerCount++
			if n.ID != me {
				t.Errorf("center node id = %q, want %q", n.ID, me)
			}
		}
	}
	if centerCount != 1 {
		t.Errorf("expected exactly 1 center node, got %d", centerCount)
	}

	// Every edge must originate at the center user.
	for _, e := range graph.Edges {
		if e.Source != me {
			t.Errorf("edge source = %q, want %q", e.Source, me)
		}
	}

	if page.ConnectionCount != 2 {
		t.Errorf("expected ConnectionCount 2, got %d", page.ConnectionCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildNetwork_NoConnections(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	const me = "user-lonely"

	mock.ExpectQuery("SELECT (.+) FROM users WHERE id =").
		WithArgs(me).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Lonely User"))

	mock.ExpectQuery("FROM connections").
		WithArgs(me).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	page, err := buildNetwork(context.Background(), db, me)
	if err != nil {
		t.Fatalf("buildNetwork returned error: %v", err)
	}

	var graph struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal([]byte(page.GraphJSON), &graph); err != nil {
		t.Fatalf("GraphJSON invalid: %v", err)
	}
	// Just the center node, no edges.
	if len(graph.Nodes) != 1 {
		t.Errorf("expected 1 node (center only), got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(graph.Edges))
	}
	if page.ConnectionCount != 0 {
		t.Errorf("expected ConnectionCount 0, got %d", page.ConnectionCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestBuildNetwork_QueryError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	const me = "user-err"
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id =").
		WithArgs(me).
		WillReturnError(sql.ErrConnDone)

	if _, err := buildNetwork(context.Background(), db, me); err == nil {
		t.Error("expected error from buildNetwork, got nil")
	}
}

// --- Template rendering ---

func TestHeatmapTemplate_RendersInlineShadedCells(t *testing.T) {
	tmpl := parsePage(t, "insights/heatmap.html")

	page := HeatmapPage{
		BaseData:    middleware.BaseData{UserID: "u1", Nav: middleware.UserInfo{Role: "manager"}},
		Departments: []string{"NRCS"},
		Skills:      []string{"GIS"},
		MaxCount:    5,
		Rows: []HeatRow{
			{Department: "NRCS", Cells: []HeatCell{{Count: 5, Color: shadeColor(5, 5)}}},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", page); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "background-color: rgb(46,125,50)") {
		t.Errorf("expected inline shaded cell style in output")
	}
	if !strings.Contains(out, "NRCS") || !strings.Contains(out, "GIS") {
		t.Errorf("expected department/skill labels in output")
	}
}

func TestNetworkTemplate_EmbedsSVGAndGraphJSON(t *testing.T) {
	tmpl := parsePage(t, "insights/network.html")

	page := NetworkPage{
		BaseData:        middleware.BaseData{UserID: "u1", Nav: middleware.UserInfo{Role: "employee"}},
		GraphJSON:       template.JS(`{"nodes":[{"id":"u1","label":"Self","center":true}],"edges":[]}`),
		ConnectionCount: 1,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", page); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "network-svg") {
		t.Errorf("expected inline <svg> stage in output")
	}
	if !strings.Contains(out, `"nodes"`) {
		t.Errorf("expected embedded graph JSON in output")
	}
	// No external library references.
	if strings.Contains(out, "d3.") || strings.Contains(out, "cdn") || strings.Contains(out, "https://unpkg") {
		t.Errorf("network template must not reference external libraries")
	}
}

func TestShadeColor(t *testing.T) {
	if got := shadeColor(0, 5); got != template.CSS("#ffffff") {
		t.Errorf("zero count should be white, got %s", got)
	}
	if got := shadeColor(3, 0); got != template.CSS("#ffffff") {
		t.Errorf("zero max should be white, got %s", got)
	}
	if got := shadeColor(5, 5); got != template.CSS("rgb(46,125,50)") {
		t.Errorf("full intensity should be USDA green, got %s", got)
	}
}
