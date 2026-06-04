package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionCookieName = "session_id"
	sessionTTL        = 24 * time.Hour
	sessionPrefix     = "session:"
	// userSessionsPrefix keys a Redis SET of session tokens belonging to a user,
	// e.g. user_sessions:<userID>. This lets us enumerate a user's active
	// sessions for the "Active Sessions" view and remote logout.
	userSessionsPrefix = "user_sessions:"
	// sessionMetaPrefix keys a Redis HASH of minimal metadata for a session
	// (created_at, user_agent), e.g. session_meta:<token>.
	sessionMetaPrefix = "session_meta:"
)

// redisStore is the subset of *redis.Client behaviour the SessionManager needs.
// Declaring it as an interface lets tests substitute an in-memory fake without
// pulling in a Redis test dependency. *redis.Client satisfies it directly.
type redisStore interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd
}

type SessionManager struct {
	redis  redisStore
	secret string
}

func secureCookieForRequest(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SECURE_COOKIES")))
	if v == "true" || v == "1" || v == "yes" {
		return true
	}
	if v == "false" || v == "0" || v == "no" {
		return false
	}

	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}

	return true
}

func NewSessionManager(redisClient *redis.Client, secret string) *SessionManager {
	return &SessionManager{
		redis:  redisClient,
		secret: secret,
	}
}

// authCtxKey is a package-private context key type for stashing the userID,
// used by the auth package's self-contained requireAuth guard.
type authCtxKey struct{}

func contextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, authCtxKey{}, userID)
}

func userIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(authCtxKey{}).(string)
	return id
}

// userSessionsKey returns the Redis SET key indexing a user's session tokens.
func userSessionsKey(userID string) string {
	return userSessionsPrefix + userID
}

// sessionMetaKey returns the Redis HASH key for a session's metadata.
func sessionMetaKey(token string) string {
	return sessionMetaPrefix + token
}

func (sm *SessionManager) Create(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := generateToken(32)
	if err != nil {
		return fmt.Errorf("generating session token: %w", err)
	}

	ctx := r.Context()
	key := sessionPrefix + token
	err = sm.redis.Set(ctx, key, userID, sessionTTL).Err()
	if err != nil {
		return fmt.Errorf("storing session: %w", err)
	}

	// Maintain a per-user index of active session tokens so they can be
	// enumerated and revoked. Best-effort: a failure here must not block login.
	if err := sm.redis.SAdd(ctx, userSessionsKey(userID), token).Err(); err != nil {
		// non-fatal: index is a convenience, the session itself already exists.
		_ = err
	}
	// Store minimal metadata for display. Best-effort.
	sm.redis.HSet(ctx, sessionMetaKey(token),
		"created_at", time.Now().UTC().Format(time.RFC3339),
		"user_agent", r.UserAgent(),
	)
	sm.redis.Expire(ctx, sessionMetaKey(token), sessionTTL)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieForRequest(r),
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

// CurrentToken returns the session token from the request cookie, if any.
func (sm *SessionManager) CurrentToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (sm *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		ctx := r.Context()
		token := cookie.Value
		// Look up the owning user so we can prune the per-user index too.
		if userID, gerr := sm.redis.Get(ctx, sessionPrefix+token).Result(); gerr == nil {
			sm.redis.SRem(ctx, userSessionsKey(userID), token)
		}
		sm.redis.Del(ctx, sessionPrefix+token, sessionMetaKey(token))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieForRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// SessionInfo describes one active session for display.
type SessionInfo struct {
	Token     string
	CreatedAt time.Time
	UserAgent string
}

// ListSessions returns the active sessions for a user, newest first. Tokens in
// the per-user index whose underlying session has expired are pruned lazily.
func (sm *SessionManager) ListSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	tokens, err := sm.redis.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("reading session index: %w", err)
	}

	sessions := make([]SessionInfo, 0, len(tokens))
	for _, token := range tokens {
		// Verify the session still exists; prune stale index entries.
		owner, gerr := sm.redis.Get(ctx, sessionPrefix+token).Result()
		if gerr == redis.Nil || owner != userID {
			sm.redis.SRem(ctx, userSessionsKey(userID), token)
			sm.redis.Del(ctx, sessionMetaKey(token))
			continue
		}
		if gerr != nil {
			return nil, fmt.Errorf("reading session: %w", gerr)
		}

		info := SessionInfo{Token: token}
		if meta, merr := sm.redis.HGetAll(ctx, sessionMetaKey(token)).Result(); merr == nil {
			if ts := meta["created_at"]; ts != "" {
				if t, perr := time.Parse(time.RFC3339, ts); perr == nil {
					info.CreatedAt = t
				}
			}
			info.UserAgent = meta["user_agent"]
		}
		sessions = append(sessions, info)
	}

	// Newest first; sessions without metadata sort last (zero time).
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// RevokeSession deletes a single session token belonging to userID, removing it
// from the session store, its metadata, and the per-user index. It is a no-op
// (returns nil) if the token does not belong to the user, so a user cannot
// revoke another user's session.
func (sm *SessionManager) RevokeSession(ctx context.Context, userID, token string) error {
	if token == "" {
		return nil
	}

	owner, err := sm.redis.Get(ctx, sessionPrefix+token).Result()
	if err == redis.Nil {
		// Session already gone; just clean the index entry.
		sm.redis.SRem(ctx, userSessionsKey(userID), token)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading session: %w", err)
	}

	if owner != userID {
		// Not this user's session: refuse silently.
		return nil
	}

	sm.redis.Del(ctx, sessionPrefix+token, sessionMetaKey(token))
	sm.redis.SRem(ctx, userSessionsKey(userID), token)
	return nil
}

func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
