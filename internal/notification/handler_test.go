package notification

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestCreateNotificationRespectsDisabledPreference verifies that when the
// recipient has a notification_preferences row with enabled=false for the
// given type, CreateNotification skips the INSERT and returns no error.
func TestCreateNotificationRespectsDisabledPreference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db, broker: NewBroker()}

	// Preference lookup returns enabled=false -> insert should be skipped.
	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("recipient-1", "kudos").
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(false))

	if err := h.CreateNotification(context.Background(), "recipient-1", "sender-1", "kudos", "msg"); err != nil {
		t.Fatalf("CreateNotification returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCreateNotificationDefaultEnabled verifies that when no preference row
// exists (the lookup returns no rows), CreateNotification treats the type as
// enabled and performs the INSERT.
func TestCreateNotificationDefaultEnabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db, broker: NewBroker()}

	// No preference row -> default enabled.
	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("recipient-2", "message").
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}))

	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs("recipient-2", "sender-2", "message", "hello").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := h.CreateNotification(context.Background(), "recipient-2", "sender-2", "message", "hello"); err != nil {
		t.Fatalf("CreateNotification returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCreateNotificationEnabledPreference verifies that an explicit
// enabled=true preference row results in the INSERT being performed.
func TestCreateNotificationEnabledPreference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db, broker: NewBroker()}

	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("recipient-3", "mention").
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))

	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs("recipient-3", "sender-3", "mention", "you were mentioned").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := h.CreateNotification(context.Background(), "recipient-3", "sender-3", "mention", "you were mentioned"); err != nil {
		t.Fatalf("CreateNotification returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleClickConnectionAccepted verifies BUG-4: a "connection_accepted"
// notification routes the clicker to the accepter's profile (sender_id), not
// the default /feed fallback. Before the fix, handleClick only matched
// "connection_accept" (missing "ed") so this fell through to /feed.
func TestHandleClickConnectionAccepted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const (
		userID   = "clicker-1"
		notifID  = "notif-1"
		accepter = "accepter-9" // stored as sender_id on the notification row
	)

	h := &Handler{db: db}

	mock.ExpectQuery(`UPDATE notifications SET read = true`).
		WithArgs(notifID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"sender_id", "type"}).
			AddRow(accepter, "connection_accepted"))

	req := httptest.NewRequest("GET", "/notifications/"+notifID+"/click", nil)
	req.SetPathValue("id", notifID)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.handleClick(rec, req)

	loc := rec.Header().Get("Location")
	want := "/profile/" + accepter
	if loc != want {
		t.Errorf("connection_accepted should route to %q, got %q", want, loc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
