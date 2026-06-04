package bookmark

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

func TestHandleAddRejectsExternalRefererRedirect(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db}

	body := "target_type=post&target_id=post-1"
	req := httptest.NewRequest("POST", "/bookmarks/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://evil.example/phish")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	mock.ExpectExec(`INSERT INTO bookmarks`).
		WithArgs("user-1", "post", "post-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h.handleAdd(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/bookmarks" {
		t.Fatalf("expected safe fallback redirect, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
