package analytics

import (
	"context"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// loadPages parses the real templates (layouts + the analytics page) the same
// way main.go's parseTemplate helper does, so the tests exercise actual
// rendering with the page data structs.
func loadPages(t *testing.T, key, file string) map[string]*template.Template {
	t.Helper()
	tmpl := template.Must(template.ParseFiles(
		"../../templates/layouts/base.html",
		"../../templates/layouts/navbar.html",
		"../../templates/analytics/"+file,
	))
	return map[string]*template.Template{key: tmpl}
}

func TestShowWorkforceAggregates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	pages := loadPages(t, "workforce_dashboard.html", "workforce_dashboard.html")
	h := NewHandler(db, pages)

	mock.MatchExpectationsInOrder(false)

	// total active employees
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	// headcount by department (join departments)
	mock.ExpectQuery(`FROM users u\s+JOIN departments d`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "count"}).
			AddRow("NRCS", 20).
			AddRow("Forest Service", 12))

	// breakdown by role
	mock.ExpectQuery(`SELECT role, COUNT\(\*\)\s+FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"role", "count"}).
			AddRow("employee", 38).
			AddRow("manager", 3).
			AddRow("admin", 1))

	// top skills
	mock.ExpectQuery(`FROM skills`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "count"}).
			AddRow("Go", 15).
			AddRow("SQL", 9))

	// engagement totals: posts, comments, connections, kudos
	mock.ExpectQuery(`FROM posts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery(`FROM comments`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(200))
	mock.ExpectQuery(`FROM connections`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))
	mock.ExpectQuery(`FROM kudos`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(30))

	req := httptest.NewRequest("GET", "/analytics/workforce", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "mgr-1"))
	rec := httptest.NewRecorder()

	h.showWorkforce(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"42", "NRCS", "Forest Service", "manager", "Go", "SQL", "100", "200", "50", "30"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected rendered dashboard to contain %q; body=%s", want, body)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestShowLeaderboardRanksByScore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	pages := loadPages(t, "leaderboard.html", "leaderboard.html")
	h := NewHandler(db, pages)

	mock.MatchExpectationsInOrder(false)

	// department options for the filter dropdown
	mock.ExpectQuery(`SELECT id, name FROM departments`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow("d1", "NRCS").
			AddRow("d2", "ARS"))

	// ranked entries
	mock.ExpectQuery(`ORDER BY score DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "department", "score"}).
			AddRow("u1", "Ada Lovelace", "NRCS", 17).
			AddRow("u2", "Grace Hopper", "ARS", 9))

	req := httptest.NewRequest("GET", "/analytics/leaderboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u9"))
	rec := httptest.NewRecorder()

	h.showLeaderboard(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Ada Lovelace", "Grace Hopper", "17", "9", "NRCS", "ARS"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected leaderboard to contain %q; body=%s", want, body)
		}
	}
	// rank 1 should appear before rank 2 contributor
	if strings.Index(body, "Ada Lovelace") > strings.Index(body, "Grace Hopper") {
		t.Errorf("expected Ada (rank 1) to render before Grace (rank 2)")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestShowLeaderboardDepartmentFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	pages := loadPages(t, "leaderboard.html", "leaderboard.html")
	h := NewHandler(db, pages)

	mock.MatchExpectationsInOrder(false)

	mock.ExpectQuery(`SELECT id, name FROM departments`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("d1", "NRCS"))

	// when ?department=d1 is set the entries query must receive d1 as an arg
	mock.ExpectQuery(`ORDER BY score DESC`).
		WithArgs("d1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "department", "score"}).
			AddRow("u1", "Ada Lovelace", "NRCS", 17))

	req := httptest.NewRequest("GET", "/analytics/leaderboard?department=d1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u9"))
	rec := httptest.NewRecorder()

	h.showLeaderboard(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
