package digest

import (
	"context"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// testPages parses the real digest template (with layouts) so handler tests
// exercise actual rendering. Paths are relative to this package directory.
func testPages() map[string]*template.Template {
	tmplDir := "../../templates"
	mk := func(name string) *template.Template {
		return template.Must(template.ParseFiles(
			tmplDir+"/layouts/base.html",
			tmplDir+"/layouts/navbar.html",
			tmplDir+"/"+name,
		))
	}
	return map[string]*template.Template{
		"digest.html": mk("digest/index.html"),
	}
}

// expectDigestQueries sets up sqlmock expectations for all five digest sections
// in the order showDigest issues them. Callers provide the rows for each.
func expectDigestQueries(mock sqlmock.Sqlmock, userID string,
	connRows, postEngRows, kudosRows *sqlmock.Rows,
	notifCount int, postingRows *sqlmock.Rows) {

	// 1. New accepted connections in last 7 days.
	mock.ExpectQuery(`FROM connections`).
		WithArgs(userID).
		WillReturnRows(connRows)

	// 2. Engagement (likes + comments) on the user's posts in last 7 days.
	mock.ExpectQuery(`FROM posts`).
		WithArgs(userID).
		WillReturnRows(postEngRows)

	// 3. Kudos received in last 7 days.
	mock.ExpectQuery(`FROM kudos`).
		WithArgs(userID).
		WillReturnRows(kudosRows)

	// 4. Unread / recent notifications count.
	mock.ExpectQuery(`FROM notifications`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(notifCount))

	// 5. New postings matching the user's skills in last 7 days.
	mock.ExpectQuery(`FROM postings`).
		WithArgs(userID).
		WillReturnRows(postingRows)
}

func newDigestRequest(userID string) *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// TestShowDigestRendersAllSections verifies the digest renders populated data
// across every section.
func TestShowDigestRendersAllSections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)

	connRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow("u-2", "Clark Kent").
		AddRow("u-3", "Diana Prince")

	postEngRows := sqlmock.NewRows([]string{"id", "content", "likes", "comments", "created_at"}).
		AddRow("p-1", "My first update about GIS work", 4, 2, now)

	kudosRows := sqlmock.NewRows([]string{"id", "sender_name", "message", "created_at"}).
		AddRow("k-1", "Bruce Wayne", "Great teamwork on the migration", now)

	postingRows := sqlmock.NewRows([]string{"id", "title", "department", "skill_name", "created_at"}).
		AddRow("post-1", "GIS Analyst Detail", "OCIO", "GIS", now)

	expectDigestQueries(mock, "user-1", connRows, postEngRows, kudosRows, 5, postingRows)

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/digest", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.showDigest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	wants := []string{
		"Clark Kent",                      // connection
		"Diana Prince",                    // connection
		"My first update about GIS work",  // post engagement
		"Bruce Wayne",                     // kudos sender
		"Great teamwork on the migration", // kudos message
		"GIS Analyst Detail",              // matching posting
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("expected digest body to contain %q, got:\n%s", w, body)
		}
	}
	// Notification count should surface.
	if !strings.Contains(body, "5") {
		t.Errorf("expected notification count 5 in digest, got:\n%s", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowDigestEmptyStates verifies each section shows an empty-state message
// when there is no data in the last 7 days.
func TestShowDigestEmptyStates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	empty := func(cols ...string) *sqlmock.Rows { return sqlmock.NewRows(cols) }

	expectDigestQueries(mock, "user-1",
		empty("id", "name"),
		empty("id", "content", "likes", "comments", "created_at"),
		empty("id", "sender_name", "message", "created_at"),
		0,
		empty("id", "title", "department", "skill_name", "created_at"),
	)

	h := NewHandler(db, testPages())

	req := httptest.NewRequest("GET", "/digest", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.showDigest(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// Each section should render its empty-state copy. We assert at least one
	// empty-state phrase per section is present.
	emptyStates := []string{
		"No new connections",
		"No engagement",
		"No kudos",
		"No new matching postings",
	}
	for _, e := range emptyStates {
		if !strings.Contains(body, e) {
			t.Errorf("expected empty-state %q in digest, got:\n%s", e, body)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
