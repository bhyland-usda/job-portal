package orgchart

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// userCols are the columns the orgchart query selects for each user.
var userCols = []string{"id", "name", "headline", "department", "manager_id"}

// TestBuildTreeNestsReports verifies the hierarchy is built from manager_id:
// employees become children of their manager, and no-manager users are roots.
func TestBuildTreeNestsReports(t *testing.T) {
	nodes := []Node{
		{ID: "1", Name: "CEO", ManagerID: ""},
		{ID: "2", Name: "VP", ManagerID: "1"},
		{ID: "3", Name: "Eng", ManagerID: "2"},
		{ID: "4", Name: "Lone", ManagerID: ""},
	}

	roots := buildTree(nodes)

	if len(roots) != 2 {
		t.Fatalf("expected 2 roots (CEO, Lone), got %d", len(roots))
	}

	var ceo *Node
	for _, r := range roots {
		if r.ID == "1" {
			ceo = r
		}
	}
	if ceo == nil {
		t.Fatal("CEO should be a root")
	}
	if len(ceo.Children) != 1 || ceo.Children[0].ID != "2" {
		t.Fatalf("CEO should have VP as its only child, got %+v", ceo.Children)
	}
	if len(ceo.Children[0].Children) != 1 || ceo.Children[0].Children[0].ID != "3" {
		t.Fatalf("VP should have Eng as its only child, got %+v", ceo.Children[0].Children)
	}
}

// TestBuildTreeHandlesCycle ensures a manager_id cycle does not cause an
// infinite loop and that cyclic members are still surfaced as roots so they are
// not silently dropped.
func TestBuildTreeHandlesCycle(t *testing.T) {
	nodes := []Node{
		{ID: "a", Name: "A", ManagerID: "b"},
		{ID: "b", Name: "B", ManagerID: "a"},
	}

	done := make(chan []*Node, 1)
	go func() { done <- buildTree(nodes) }()

	roots := <-done // if buildTree loops forever the test times out and fails
	if len(roots) == 0 {
		t.Fatal("cyclic users must still be reachable as roots")
	}
}

// TestShowChartQueriesUsers verifies showChart queries the users table and
// renders without error (template map is nil-safe via the guard).
func TestShowChartQueriesUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM users`).
		WillReturnRows(sqlmock.NewRows(userCols).
			AddRow("1", "Ada Lovelace", "Engineer", "ARS", nil).
			AddRow("2", "Bob Smith", "Analyst", "FSA", "1"))

	h := NewHandler(db, nil)

	req := httptest.NewRequest("GET", "/orgchart", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "1"))
	rec := httptest.NewRecorder()

	h.showChart(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestAssignRejectsSelfManager verifies a user cannot be set as their own
// manager: no UPDATE is issued and the response is a client error.
func TestAssignRejectsSelfManager(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// No ExpectExec is registered: if the handler issues an UPDATE the mock
	// will fail because the call was not expected.
	h := NewHandler(db, nil)

	form := url.Values{}
	form.Set("employee_id", "u1")
	form.Set("manager_id", "u1")

	req := httptest.NewRequest("POST", "/admin/orgchart", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin"))
	rec := httptest.NewRecorder()

	h.handleAssign(rec, req)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("self-manager should be a 4xx client error, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected db calls for self-manager: %v", err)
	}
}

// TestAssignSetsManager verifies a valid assignment issues the UPDATE and
// redirects to /orgchart.
func TestAssignSetsManager(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE users SET manager_id`).
		WithArgs("mgr1", "emp1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, nil)

	form := url.Values{}
	form.Set("employee_id", "emp1")
	form.Set("manager_id", "mgr1")

	req := httptest.NewRequest("POST", "/admin/orgchart", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin"))
	rec := httptest.NewRecorder()

	h.handleAssign(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/orgchart" {
		t.Errorf("expected redirect to /orgchart, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
