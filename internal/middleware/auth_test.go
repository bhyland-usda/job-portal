package middleware

import (
	"context"
	"testing"
)

func TestGetUserIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "test-user-123")
	userID := GetUserID(ctx)
	if userID != "test-user-123" {
		t.Errorf("Expected 'test-user-123', got '%s'", userID)
	}
}

func TestGetUserIDEmptyContext(t *testing.T) {
	userID := GetUserID(context.Background())
	if userID != "" {
		t.Errorf("Expected empty string from empty context, got '%s'", userID)
	}
}

func TestGetUserInfoFromContext(t *testing.T) {
	info := UserInfo{
		ID:        "user-1",
		FirstName: "Bryan",
		LastName:  "Hyland",
		Initials:  "BH",
		Role:      "admin",
		AvatarURL: "/avatar/user-1",
	}
	ctx := context.WithValue(context.Background(), UserInfoKey, info)
	result := GetUserInfo(ctx)

	if result.ID != "user-1" {
		t.Errorf("Expected ID 'user-1', got '%s'", result.ID)
	}
	if result.FirstName != "Bryan" {
		t.Errorf("Expected FirstName 'Bryan', got '%s'", result.FirstName)
	}
	if result.Initials != "BH" {
		t.Errorf("Expected Initials 'BH', got '%s'", result.Initials)
	}
	if result.Role != "admin" {
		t.Errorf("Expected Role 'admin', got '%s'", result.Role)
	}
}

func TestGetUserInfoEmptyContext(t *testing.T) {
	result := GetUserInfo(context.Background())
	if result.ID != "" {
		t.Errorf("Expected empty UserInfo from empty context, got ID '%s'", result.ID)
	}
}

func TestBaseDataCreation(t *testing.T) {
	// BaseData should have both UserID and Nav populated
	bd := BaseData{
		UserID: "user-1",
		Nav: UserInfo{
			ID:        "user-1",
			FirstName: "Test",
			LastName:  "User",
			Initials:  "TU",
			Role:      "employee",
		},
	}

	if bd.UserID != bd.Nav.ID {
		t.Error("BaseData.UserID should match Nav.ID")
	}
	if bd.Nav.Initials != "TU" {
		t.Errorf("Expected initials 'TU', got '%s'", bd.Nav.Initials)
	}
}
