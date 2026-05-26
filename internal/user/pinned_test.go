package user

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestPinPostOwnPostInserts verifies pinning a post you own performs the
// ownership check and then inserts the pin row (idempotently).
func TestPinPostOwnPostInserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT user_id FROM posts WHERE id = ").
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user-1"))
	mock.ExpectExec("INSERT INTO pinned_profile_posts").
		WithArgs("user-1", "post-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	ok, err := pinPost(context.Background(), db, "user-1", "post-1")
	if err != nil {
		t.Fatalf("pinPost returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected pinPost to succeed for owned post")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestPinPostNotOwnerRejected verifies pinning a post you do NOT own is rejected
// (returns ok=false) and never reaches the INSERT.
func TestPinPostNotOwnerRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT user_id FROM posts WHERE id = ").
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("someone-else"))
	// No ExpectExec: an INSERT would be an unexpected call and fail the test.

	ok, err := pinPost(context.Background(), db, "user-1", "post-1")
	if err != nil {
		t.Fatalf("pinPost returned error: %v", err)
	}
	if ok {
		t.Fatal("expected pinPost to reject a post the user does not own")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestPinPostMissingPostRejected verifies a non-existent post is rejected.
func TestPinPostMissingPostRejected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT user_id FROM posts WHERE id = ").
		WithArgs("ghost").
		WillReturnError(sql.ErrNoRows)

	ok, err := pinPost(context.Background(), db, "user-1", "ghost")
	if err != nil {
		t.Fatalf("pinPost returned error: %v", err)
	}
	if ok {
		t.Fatal("expected pinPost to reject a non-existent post")
	}
}

// TestUnpinPostDeletes verifies unpin deletes the row for the current user.
func TestUnpinPostDeletes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("DELETE FROM pinned_profile_posts WHERE user_id = .+ AND post_id = ").
		WithArgs("user-1", "post-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := unpinPost(context.Background(), db, "user-1", "post-1"); err != nil {
		t.Fatalf("unpinPost returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
