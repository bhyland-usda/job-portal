package user

import (
	"net/http/httptest"
	"testing"
)

func TestServeAvatarRequiresAuthContext(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/avatar/u1", nil)
	req.SetPathValue("id", "u1")
	rec := httptest.NewRecorder()

	h.ServeAvatar(rec, req)

	if rec.Code != 401 {
		t.Fatalf("expected 401 for unauthenticated request, got %d", rec.Code)
	}
}
