package badge

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestCheckAndAwardFirstPost verifies CheckAndAward inserts the "First Post"
// badge when the user has at least one post, and that re-running it does not
// create a duplicate (idempotent thanks to ON CONFLICT DO NOTHING).
func TestCheckAndAwardFirstPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-123"

	// expectAwardRun sets up expectations for a single CheckAndAward pass.
	// Only the "First Post" check returns true; the rest return false so no
	// further inserts happen. The Team Player check errors (table missing)
	// and is skipped silently.
	expectAwardRun := func() {
		// First Post -> exists
		mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM posts`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		// First Post award insert
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_badges`)).
			WithArgs(userID, "First Post").
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Connector -> not met
		mock.ExpectQuery(`FROM connections`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))

		// Profile Complete -> not met
		mock.ExpectQuery(`FROM users u WHERE u.id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))

		// Team Player -> table error, skipped
		mock.ExpectQuery(`FROM kudos`).
			WithArgs(userID).
			WillReturnError(context.DeadlineExceeded)
	}

	// First pass: awards the badge.
	expectAwardRun()
	CheckAndAward(context.Background(), db, userID)

	// Second pass: idempotent. The INSERT runs again but ON CONFLICT DO NOTHING
	// means 0 rows affected; no duplicate badge is created.
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM posts`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_badges`)).
		WithArgs(userID, "First Post").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows: already earned
	mock.ExpectQuery(`FROM connections`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))
	mock.ExpectQuery(`FROM users u WHERE u.id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))
	mock.ExpectQuery(`FROM kudos`).
		WithArgs(userID).
		WillReturnError(context.DeadlineExceeded)

	CheckAndAward(context.Background(), db, userID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCheckAndAwardDetailAndMentor verifies the "Detail Completed" and "Mentor"
// checks award their badges when the conditions are met. The first four checks
// return not-met so only Detail Completed and Mentor are awarded.
func TestCheckAndAwardDetailAndMentor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	const userID = "user-123"

	// First four checks: not met (no inserts).
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM posts`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`FROM connections`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))
	mock.ExpectQuery(`FROM users u WHERE u.id`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))
	mock.ExpectQuery(`FROM kudos`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"met"}).AddRow(false))

	// Detail Completed -> met, awarded.
	mock.ExpectQuery(`FROM posting_applications`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_badges`)).
		WithArgs(userID, "Detail Completed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mentor -> met, awarded.
	mock.ExpectQuery(`FROM mentorships`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO user_badges`)).
		WithArgs(userID, "Mentor").
		WillReturnResult(sqlmock.NewResult(0, 1))

	CheckAndAward(context.Background(), db, userID)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
