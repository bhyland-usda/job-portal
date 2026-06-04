package messaging

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestStartGroupConversationStoresName verifies that when creating a GROUP
// (more than two participants), the optional `name` form field is persisted to
// conversations.name. The INSERT must carry the name; the 1:1 dedupe path is
// untouched (a group sets direct_key NULL and is unconstrained).
func TestStartGroupConversationStoresName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	const me, u1, u2, convID, groupName = "user-a", "user-b", "user-c", "conv-1", "Project Team"

	mock.ExpectBegin()
	// The group create must INSERT a name alongside direct_key (NULL for groups).
	mock.ExpectQuery(`INSERT INTO conversations`).
		WithArgs(nil, groupName).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(convID))
	// Current user added as participant.
	mock.ExpectExec(`INSERT INTO conversation_participants`).
		WithArgs(convID, me).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Each selected user added.
	mock.ExpectExec(`INSERT INTO conversation_participants`).
		WithArgs(convID, u1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO conversation_participants`).
		WithArgs(convID, u2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	h := &Handler{db: db}

	body := "user_ids=" + u1 + "&user_ids=" + u2 + "&name=" + strings.ReplaceAll(groupName, " ", "+")
	req := httptest.NewRequest("POST", "/messages/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, me))
	rec := httptest.NewRecorder()

	h.startGroupConversation(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), convID) {
		t.Errorf("expected response to contain conversation id %q, got %s", convID, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestAddMemberRejectsNonParticipant verifies addMember refuses a requester who
// is not already a participant of the conversation, and performs no insert.
func TestAddMemberRejectsNonParticipant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	const me, target, convID = "outsider", "user-x", "conv-1"

	// isParticipant check returns false -> handler must forbid before any insert.
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(convID, me).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	h := &Handler{db: db}

	body := "user_id=" + target
	req := httptest.NewRequest("POST", "/messages/"+convID+"/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", convID)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, me))
	rec := httptest.NewRecorder()

	h.addMember(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403 for non-participant, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestAddMemberInsertsForParticipant verifies addMember adds the user when the
// requester is already a participant, using ON CONFLICT DO NOTHING.
func TestAddMemberInsertsForParticipant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	const me, target, convID = "insider", "user-x", "conv-1"

	// isParticipant check returns true -> handler proceeds to insert.
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(convID, me).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`INSERT INTO conversation_participants`).
		WithArgs(convID, target).
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := &Handler{db: db}

	body := "user_id=" + target
	req := httptest.NewRequest("POST", "/messages/"+convID+"/members", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", convID)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, me))
	rec := httptest.NewRecorder()

	h.addMember(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStartGroupConversationParticipantInsertFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	const me, u1, convID = "user-a", "user-b", "conv-1"

	mock.ExpectQuery(`SELECT cp1\.conversation_id`).
		WithArgs(me, u1).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO conversations`).
		WithArgs("user-a-user-b", nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(convID))
	// Current user insert fails -> should return 500 and rollback.
	mock.ExpectExec(`INSERT INTO conversation_participants`).
		WithArgs(convID, me).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	h := &Handler{db: db}
	body := "user_ids=" + u1
	req := httptest.NewRequest("POST", "/messages/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, me))
	rec := httptest.NewRecorder()

	h.startGroupConversation(rec, req)

	if rec.Code != 500 {
		t.Fatalf("expected 500, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
