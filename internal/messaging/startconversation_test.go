package messaging

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
)

// TestStartConversationOnlyMatchesDirectConversation is a regression test for the
// bug where a 1:1 message was routed into an existing GROUP conversation that
// happened to include both users. The find-existing lookup must restrict to a
// conversation with exactly two participants (a true direct conversation), which
// the query expresses via a participant-count subquery (alias cp3, count = 2).
func TestStartConversationOnlyMatchesDirectConversation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	const me, other, directID = "user-a", "user-b", "direct-1"

	// The lookup MUST constrain to exactly-2-participant conversations. Matching
	// on the participant-count subquery proves the constraint is present; we then
	// return an existing direct conversation so the handler echoes its id.
	mock.ExpectQuery(`conversation_participants cp3`).
		WithArgs(me, other).
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id"}).AddRow(directID))

	h := &Handler{db: db}

	req := httptest.NewRequest("POST", "/messages/new/"+other, nil)
	req.SetPathValue("userID", other)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, me))
	rec := httptest.NewRecorder()

	h.startConversation(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), directID) {
		t.Errorf("expected response to contain direct conversation id %q, got %s", directID, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestDirectKeyIsOrderIndependent verifies the canonical direct-conversation key
// is order-independent and matches migration 048's backfill format (the two ids
// sorted ascending, joined by '-'), so the partial unique index dedupes a pair
// regardless of who initiates.
func TestDirectKeyIsOrderIndependent(t *testing.T) {
	if got := directKey("b", "a"); got != "a-b" {
		t.Errorf("directKey(\"b\",\"a\") = %q, want \"a-b\"", got)
	}
	if directKey("a", "b") != directKey("b", "a") {
		t.Error("directKey must be order-independent")
	}
}
