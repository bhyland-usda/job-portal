package dataexport

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestDownloadServesJSONWithProfileAndPosts verifies the /data-export/download
// endpoint aggregates the current user's own data and serves it as a JSON
// attachment. We assert the response headers (application/json + attachment)
// and that the profile and posts sections are populated from the DB.
func TestDownloadServesJSONWithProfileAndPosts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Sections are queried in an order we don't want the test coupled to.
	mock.MatchExpectationsInOrder(false)

	now := time.Now()
	userID := "user-123"

	// Profile (users row, no password_hash).
	mock.ExpectQuery(`FROM users`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "headline",
			"about", "avatar_url", "location", "created_at", "updated_at",
		}).AddRow(userID, "jane@example.gov", "Jane", "Doe", "Engineer",
			"About Jane", "", "DC", now, now))

	// Posts.
	mock.ExpectQuery(`FROM posts`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "created_at"}).
			AddRow("post-1", "hello world", now).
			AddRow("post-2", "second post", now))

	// Comments.
	mock.ExpectQuery(`FROM comments`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "post_id", "content", "created_at"}))

	// Connections.
	mock.ExpectQuery(`FROM connections`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "requester_id", "addressee_id", "status", "created_at"}))

	// Kudos sent.
	mock.ExpectQuery(`FROM kudos.*sender_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "receiver_id", "message", "created_at"}))

	// Kudos received.
	mock.ExpectQuery(`FROM kudos.*receiver_id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sender_id", "message", "created_at"}))

	// Accomplishments.
	mock.ExpectQuery(`FROM accomplishments`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "period_type", "period_start", "period_end", "created_at"}))

	h := NewHandler(db, nil)

	req := httptest.NewRequest("GET", "/data-export/download", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.downloadData(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="my-data.json"` {
		t.Errorf("unexpected Content-Disposition: %q", cd)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json export: %v; body=%s", err, rec.Body.String())
	}

	// Profile section must be populated.
	var profile map[string]interface{}
	if err := json.Unmarshal(doc["profile"], &profile); err != nil {
		t.Fatalf("profile section not valid object: %v", err)
	}
	if profile["email"] != "jane@example.gov" {
		t.Errorf("expected profile email jane@example.gov, got %v", profile["email"])
	}
	if _, leaked := profile["password_hash"]; leaked {
		t.Error("password_hash must NOT be present in export")
	}

	// Posts section must contain the two rows.
	var posts []map[string]interface{}
	if err := json.Unmarshal(doc["posts"], &posts); err != nil {
		t.Fatalf("posts section not valid array: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d: %v", len(posts), posts)
	}
}

// TestDownloadBestEffortOnSectionError verifies that a query error in one
// section (here: posts) does not fail the whole export. The profile section
// should still be present and the document still valid JSON.
func TestDownloadBestEffortOnSectionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.MatchExpectationsInOrder(false)
	now := time.Now()
	userID := "user-err"

	mock.ExpectQuery(`FROM users`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "first_name", "last_name", "headline",
			"about", "avatar_url", "location", "created_at", "updated_at",
		}).AddRow(userID, "x@example.gov", "X", "Y", "", "", "", "", now, now))

	// Posts query errors out.
	mock.ExpectQuery(`FROM posts`).
		WithArgs(userID).
		WillReturnError(context.DeadlineExceeded)

	// Remaining sections still queried; return empty.
	mock.ExpectQuery(`FROM comments`).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "post_id", "content", "created_at"}))
	mock.ExpectQuery(`FROM connections`).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "requester_id", "addressee_id", "status", "created_at"}))
	mock.ExpectQuery(`FROM kudos.*sender_id`).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "receiver_id", "message", "created_at"}))
	mock.ExpectQuery(`FROM kudos.*receiver_id`).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sender_id", "message", "created_at"}))
	mock.ExpectQuery(`FROM accomplishments`).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "period_type", "period_start", "period_end", "created_at"}))

	h := NewHandler(db, nil)

	req := httptest.NewRequest("GET", "/data-export/download", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()

	h.downloadData(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200 despite section error, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json export: %v", err)
	}
	if _, ok := doc["profile"]; !ok {
		t.Error("profile section missing despite best-effort export")
	}
}
