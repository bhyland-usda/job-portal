package certification

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bhyland-usda/job-portal/internal/middleware"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	return db, mock
}

// authedRequest builds a request whose context carries the given user ID,
// mirroring what requireAuth middleware injects.
func authedRequest(method, target, userID string, body interface{}) *http.Request {
	var r *http.Request
	if s, ok := body.(string); ok {
		r = httptest.NewRequest(method, target, strings.NewReader(s))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		expires sql.NullTime
		want    string
	}{
		{"no expiry is active", sql.NullTime{Valid: false}, "Active"},
		{"expired yesterday", sql.NullTime{Time: now.AddDate(0, 0, -1), Valid: true}, "Expired"},
		{"expires today is expiring soon", sql.NullTime{Time: now, Valid: true}, "Expiring soon"},
		{"expires in 10 days is expiring soon", sql.NullTime{Time: now.AddDate(0, 0, 10), Valid: true}, "Expiring soon"},
		{"expires in exactly 90 days is expiring soon", sql.NullTime{Time: now.AddDate(0, 0, 90), Valid: true}, "Expiring soon"},
		{"expires in 91 days is active", sql.NullTime{Time: now.AddDate(0, 0, 91), Valid: true}, "Active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeStatus(tt.expires, now)
			if got != tt.want {
				t.Errorf("computeStatus(%v) = %q, want %q", tt.expires, got, tt.want)
			}
		})
	}
}

func TestShowCertificationsQuery(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	userID := "11111111-1111-1111-1111-111111111111"

	rows := sqlmock.NewRows([]string{"id", "name", "issuer", "issued_on", "expires_on", "created_at"}).
		AddRow("c1", "Security+", sql.NullString{String: "CompTIA", Valid: true},
			sql.NullTime{Valid: false},
			sql.NullTime{Time: time.Now().AddDate(0, 0, 10), Valid: true},
			time.Now())

	mock.ExpectQuery("SELECT id, name, issuer, issued_on, expires_on, created_at FROM certifications").
		WithArgs(userID).
		WillReturnRows(rows)

	h := NewHandler(db, nil)
	// We can't render a template (pages is nil), so call the data loader directly.
	certs, err := h.getUserCertifications(context.Background(), userID)
	if err != nil {
		t.Fatalf("getUserCertifications returned error: %v", err)
	}
	if len(certs) != 1 {
		t.Fatalf("expected 1 cert, got %d", len(certs))
	}
	if certs[0].Name != "Security+" {
		t.Errorf("expected name Security+, got %q", certs[0].Name)
	}
	if certs[0].Issuer != "CompTIA" {
		t.Errorf("expected issuer CompTIA, got %q", certs[0].Issuer)
	}
	if certs[0].Status != "Expiring soon" {
		t.Errorf("expected status 'Expiring soon', got %q", certs[0].Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleCreateInsertsWithNullDates(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	userID := "22222222-2222-2222-2222-222222222222"

	mock.ExpectExec("INSERT INTO certifications").
		WithArgs(userID, "PMP", sql.NullString{Valid: false}, sql.NullTime{Valid: false}, sql.NullTime{Valid: false}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := NewHandler(db, nil)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/certifications", userID, "name=PMP")

	h.handleCreate(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleCreateRequiresName(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	userID := "22222222-2222-2222-2222-222222222222"

	h := NewHandler(db, nil)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/certifications", userID, "name=")

	h.handleCreate(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303 even on validation failure, got %d", w.Code)
	}
	// No INSERT should have been issued.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleCreateWithDates(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	userID := "33333333-3333-3333-3333-333333333333"

	issued := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO certifications").
		WithArgs(userID, "CISSP", sql.NullString{String: "ISC2", Valid: true},
			sql.NullTime{Time: issued, Valid: true},
			sql.NullTime{Time: expires, Valid: true}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := NewHandler(db, nil)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/certifications", userID,
		"name=CISSP&issuer=ISC2&issued_on=2024-01-15&expires_on=2027-01-15")

	h.handleCreate(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleDeleteScopedToUser(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	userID := "44444444-4444-4444-4444-444444444444"
	certID := "cert-abc"

	mock.ExpectExec("DELETE FROM certifications").
		WithArgs(certID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewHandler(db, nil)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/certifications/"+certID+"/delete", userID, nil)
	r.SetPathValue("id", certID)

	h.handleDelete(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
