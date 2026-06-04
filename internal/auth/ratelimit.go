package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
}

type loginRateLimiter struct {
	mu           sync.Mutex
	attempts     map[string]loginAttempt
	maxFailures  int
	lockDuration time.Duration
	now          func() time.Time
}

func newLoginRateLimiter(maxFailures int, lockDuration time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		attempts:     map[string]loginAttempt{},
		maxFailures:  maxFailures,
		lockDuration: lockDuration,
		now:          time.Now,
	}
}

func (l *loginRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	a := l.attempts[key]
	n := l.now()
	if !a.lockedUntil.IsZero() && n.Before(a.lockedUntil) {
		return false
	}
	if !a.lockedUntil.IsZero() && !n.Before(a.lockedUntil) {
		a.failures = 0
		a.lockedUntil = time.Time{}
		l.attempts[key] = a
	}
	return true
}

func (l *loginRateLimiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	a := l.attempts[key]
	a.failures++
	if a.failures >= l.maxFailures {
		a.lockedUntil = l.now().Add(l.lockDuration)
	}
	l.attempts[key] = a
}

func (l *loginRateLimiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func loginRateLimitKey(r *http.Request, email string) string {
	ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if ip != "" {
		if i := strings.Index(ip, ","); i != -1 {
			ip = strings.TrimSpace(ip[:i])
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
		if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = h
		}
	}
	if ip == "" {
		ip = "unknown"
	}
	return strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}
