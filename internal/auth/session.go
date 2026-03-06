package auth

import(
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionCookieName = "session_id"
	sessionTTL        = 24 * time.Hour
	sessionPrefix     = "session:"
)

type SessionManager struct {
	redis  *redis.Client
	secret string
}

func NewSessionManager(redisClient *redis.Client, secret string) *SessionManager {
	return &SessionManager {
		redis: redisClient,
		secret: secret,
	}
}

func (sm *SessionManager) Create(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := generateToken(32)
	if err != nil {
		return fmt.Errorf("generating session token: %w", err)
	}

	key := sessionPrefix + token
	err = sm.redis.Set(r.Context(), key, userID, sessionTTL).Err()
	if err != nil {
		return fmt.Errorf("storing session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	return nil
}

func (sm *SessionManager) GetUserID(ctx context.Context, r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", err
	}

	key := sessionPrefix + cookie.Value
	userID, err := sm.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", http.ErrNoCookie
	}

	if err != nil {
		return "", fmt.Errorf("reading session: %w", err)
	}

	// Extend session on activity
	sm.redis.Expire(ctx, key, sessionTTL)

	return userID, nil
}

func (sm *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		sm.redis.Del(r.Context(), sessionPrefix + cookie.Value)
	}

	http.SetCookie(w, &http.Cookie {
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
