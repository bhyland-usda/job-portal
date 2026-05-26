package auth

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	return db, mock
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"short", false},
		{"password123", true},
		{"12345678", true},
		{"1234567", false},
		{"", false},
	}

	for _, tt := range tests {
		got := len(tt.password) >= 8
		if got != tt.valid {
			t.Errorf("password %q: expected valid=%v, got %v", tt.password, tt.valid, got)
		}
	}
}

func TestBcryptHashComparison(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrongpassword")); err == nil {
		t.Error("expected wrong password to fail comparison")
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte("correctpassword")); err != nil {
		t.Errorf("expected correct password to succeed: %v", err)
	}
}

func TestLoginQuery(t *testing.T) {
	db, mock := setupAuthMockDB(t)
	defer db.Close()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	mock.ExpectQuery("SELECT id, password_hash FROM users").
		WithArgs("test@usda.gov").
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).
			AddRow("user-123", string(hash)))

	var userID, dbHash string
	err := db.QueryRow("SELECT id, password_hash FROM users where email = ", "test@usda.gov").
		Scan(&userID, &dbHash)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if userID != "user-123" {
		t.Errorf("expected user-123, got %s", userID)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbHash), []byte("password123")); err != nil {
		t.Errorf("password comparison failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestLoginQuery_UserNotFound(t *testing.T) {
	db, mock := setupAuthMockDB(t)
	defer db.Close()

	mock.ExpectQuery("SELECT id, password_hash FROM users").
		WithArgs("nobody@usda.gov").
		WillReturnError(sql.ErrNoRows)

	var userID string
	err := db.QueryRow("SELECT id, password_hash FROM users where email = ", "nobody@usda.gov").
		Scan(&userID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
