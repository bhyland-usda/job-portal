package foia

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestExportDataTypeFilterPost verifies BUG-4: exportData with type=post must
// query only the posts table and return only post rows. Previously it ignored
// the `type` param and always exported posts+comments+messages.
func TestExportDataTypeFilterPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)

	// Make matching order-independent and make all three content tables return a
	// distinct row. If exportData honors type=post it queries ONLY posts (1 row);
	// if it ignores the filter it pulls all three (3 rows). We assert on the
	// resulting JSON rather than expectation bookkeeping so the test is precise
	// about behavior, not call order.
	mock.MatchExpectationsInOrder(false)

	now := time.Now()
	mock.ExpectQuery(`FROM posts p`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a post", "Ada Lovelace", now))
	mock.ExpectQuery(`FROM comments c`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a comment", "Bob", now))
	mock.ExpectQuery(`FROM messages m`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a message", "Cy", now))

	req := httptest.NewRequest("GET", "/admin/foia/export?type=post", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.exportData(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("invalid json export: %v; body=%s", err, rec.Body.String())
	}
	if len(rows) != 1 {
		t.Fatalf("type=post should export only posts; expected 1 row, got %d: %v", len(rows), rows)
	}
	if rows[0]["type"] != "post" {
		t.Errorf("expected type 'post', got %v", rows[0]["type"])
	}
}

// TestExportDataNoTypeExportsAll verifies that with no type param, all three
// content types are queried (backwards-compatible "all" behavior).
func TestExportDataNoTypeExportsAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := NewHandler(db, nil)
	now := time.Now()

	mock.ExpectQuery(`FROM posts p`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a post", "Ada", now))
	mock.ExpectQuery(`FROM comments c`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a comment", "Bob", now))
	mock.ExpectQuery(`FROM messages m`).
		WillReturnRows(sqlmock.NewRows([]string{"content", "author", "created_at"}).
			AddRow("a message", "Cy", now))

	req := httptest.NewRequest("GET", "/admin/foia/export", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "admin-1"))
	rec := httptest.NewRecorder()

	h.exportData(rec, req)

	var rows []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("invalid json export: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (all types), got %d", len(rows))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
