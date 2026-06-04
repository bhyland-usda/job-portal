package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginRateLimiterLocksAndResets(t *testing.T) {
	l := newLoginRateLimiter(3, 5*time.Minute)
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	key := "user@example.com|1.2.3.4"
	if !l.Allow(key) {
		t.Fatal("expected initial allow")
	}

	l.Failure(key)
	l.Failure(key)
	l.Failure(key)

	if l.Allow(key) {
		t.Fatal("expected lockout after max failures")
	}

	now = now.Add(6 * time.Minute)
	if !l.Allow(key) {
		t.Fatal("expected lockout to expire after duration")
	}
}

func TestLoginRateLimitKeyUsesForwardedFor(t *testing.T) {
	req := httptest.NewRequest("POST", "/login", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 10.0.0.10")
	key := loginRateLimitKey(req, "User@Example.com")
	if key != "user@example.com|10.0.0.5" {
		t.Fatalf("unexpected key: %q", key)
	}
}
