package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeRedis is an in-memory implementation of the redisStore interface used by
// SessionManager. It avoids a real Redis dependency (no miniredis in the module
// graph) while exercising the actual list/revoke/index bookkeeping logic.
//
// It supports the subset of behaviour SessionManager relies on: string keys
// (Set/Get/Del), set membership (SAdd/SRem/SMembers) and hashes (HSet/HGetAll).
type fakeRedis struct {
	strs map[string]string
	sets map[string]map[string]struct{}
	hash map[string]map[string]string
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		strs: map[string]string{},
		sets: map[string]map[string]struct{}{},
		hash: map[string]map[string]string{},
	}
}

func (f *fakeRedis) Set(ctx context.Context, key string, value interface{}, _ time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	f.strs[key] = toStr(value)
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if v, ok := f.strs[key]; ok {
		cmd.SetVal(v)
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (f *fakeRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var n int64
	for _, k := range keys {
		if _, ok := f.strs[k]; ok {
			delete(f.strs, k)
			n++
		}
		if _, ok := f.sets[k]; ok {
			delete(f.sets, k)
			n++
		}
		if _, ok := f.hash[k]; ok {
			delete(f.hash, k)
			n++
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) Expire(ctx context.Context, _ string, _ time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}

func (f *fakeRedis) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if f.sets[key] == nil {
		f.sets[key] = map[string]struct{}{}
	}
	var n int64
	for _, m := range members {
		s := toStr(m)
		if _, ok := f.sets[key][s]; !ok {
			f.sets[key][s] = struct{}{}
			n++
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var n int64
	if set, ok := f.sets[key]; ok {
		for _, m := range members {
			s := toStr(m)
			if _, ok := set[s]; ok {
				delete(set, s)
				n++
			}
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	var out []string
	for m := range f.sets[key] {
		out = append(out, m)
	}
	sort.Strings(out)
	cmd.SetVal(out)
	return cmd
}

func (f *fakeRedis) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if f.hash[key] == nil {
		f.hash[key] = map[string]string{}
	}
	for i := 0; i+1 < len(values); i += 2 {
		f.hash[key][toStr(values[i])] = toStr(values[i+1])
	}
	cmd.SetVal(int64(len(values) / 2))
	return cmd
}

func (f *fakeRedis) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	cmd := redis.NewMapStringStringCmd(ctx)
	out := map[string]string{}
	for k, v := range f.hash[key] {
		out[k] = v
	}
	cmd.SetVal(out)
	return cmd
}

func toStr(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// newTestManager returns a SessionManager backed by the in-memory fake.
func newTestManager() (*SessionManager, *fakeRedis) {
	fr := newFakeRedis()
	return &SessionManager{redis: fr, secret: "test"}, fr
}

// createSessionFor runs Create and returns the issued token from the Set-Cookie.
func createSessionFor(t *testing.T, sm *SessionManager, userID string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	if err := sm.Create(rec, req, userID); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c.Value
		}
	}
	t.Fatal("no session cookie set by Create")
	return ""
}

func TestCreateAddsToUserIndex(t *testing.T) {
	sm, fr := newTestManager()
	token := createSessionFor(t, sm, "user-1")

	// Session value stored.
	if got := fr.strs[sessionPrefix+token]; got != "user-1" {
		t.Errorf("session value = %q, want user-1", got)
	}
	// Token present in the per-user index set.
	set, ok := fr.sets[userSessionsKey("user-1")]
	if !ok {
		t.Fatalf("user index set not created")
	}
	if _, ok := set[token]; !ok {
		t.Errorf("token %q not in user index", token)
	}
	// Metadata captured.
	meta := fr.hash[sessionMetaKey(token)]
	if meta["user_agent"] != "test-agent/1.0" {
		t.Errorf("user_agent meta = %q, want test-agent/1.0", meta["user_agent"])
	}
	if meta["created_at"] == "" {
		t.Errorf("created_at meta missing")
	}
}

func TestListSessionsReturnsAllForUser(t *testing.T) {
	sm, _ := newTestManager()
	t1 := createSessionFor(t, sm, "user-1")
	t2 := createSessionFor(t, sm, "user-1")
	other := createSessionFor(t, sm, "user-2")

	sessions, err := sm.ListSessions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}

	tokens := map[string]bool{}
	for _, s := range sessions {
		tokens[s.Token] = true
	}
	if !tokens[t1] || !tokens[t2] {
		t.Errorf("listed sessions missing one of the user's tokens: %v", tokens)
	}
	if tokens[other] {
		t.Errorf("listed sessions leaked another user's token %q", other)
	}
}

func TestRevokeSessionRemovesSessionAndIndex(t *testing.T) {
	sm, fr := newTestManager()
	keep := createSessionFor(t, sm, "user-1")
	revoke := createSessionFor(t, sm, "user-1")

	if err := sm.RevokeSession(context.Background(), "user-1", revoke); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	// Revoked session gone from store, metadata and index.
	if _, ok := fr.strs[sessionPrefix+revoke]; ok {
		t.Errorf("revoked session still in store")
	}
	if _, ok := fr.hash[sessionMetaKey(revoke)]; ok {
		t.Errorf("revoked session metadata still present")
	}
	if _, ok := fr.sets[userSessionsKey("user-1")][revoke]; ok {
		t.Errorf("revoked token still in user index")
	}

	// The other session is untouched.
	if _, ok := fr.strs[sessionPrefix+keep]; !ok {
		t.Errorf("non-revoked session was removed")
	}

	sessions, _ := sm.ListSessions(context.Background(), "user-1")
	if len(sessions) != 1 || sessions[0].Token != keep {
		t.Errorf("after revoke, expected only %q, got %+v", keep, sessions)
	}
}

func TestRevokeSessionRejectsOtherUsersToken(t *testing.T) {
	sm, fr := newTestManager()
	victimToken := createSessionFor(t, sm, "victim")
	_ = createSessionFor(t, sm, "attacker")

	// Attacker attempts to revoke the victim's session.
	if err := sm.RevokeSession(context.Background(), "attacker", victimToken); err != nil {
		t.Fatalf("RevokeSession returned error: %v", err)
	}

	// Victim's session must survive.
	if _, ok := fr.strs[sessionPrefix+victimToken]; !ok {
		t.Errorf("attacker was able to revoke victim's session")
	}
	if _, ok := fr.sets[userSessionsKey("victim")][victimToken]; !ok {
		t.Errorf("victim's token removed from their own index")
	}
}

func TestListSessionsPrunesStaleIndexEntries(t *testing.T) {
	sm, fr := newTestManager()
	live := createSessionFor(t, sm, "user-1")

	// Simulate an expired session: present in the index but missing from the
	// session store (TTL elapsed).
	stale := "deadbeef00000000deadbeef00000000"
	fr.sets[userSessionsKey("user-1")][stale] = struct{}{}

	sessions, err := sm.ListSessions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Token != live {
		t.Fatalf("expected only live session, got %+v", sessions)
	}
	// Stale entry pruned from the index.
	if _, ok := fr.sets[userSessionsKey("user-1")][stale]; ok {
		t.Errorf("stale index entry was not pruned")
	}
}

func TestDestroyRemovesFromIndex(t *testing.T) {
	sm, fr := newTestManager()
	token := createSessionFor(t, sm, "user-1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	sm.Destroy(rec, req)

	if _, ok := fr.strs[sessionPrefix+token]; ok {
		t.Errorf("Destroy did not delete session from store")
	}
	if _, ok := fr.sets[userSessionsKey("user-1")][token]; ok {
		t.Errorf("Destroy did not remove token from user index")
	}
}

func TestSecureCookieForRequest_DefaultsByHost(t *testing.T) {
	t.Setenv("SECURE_COOKIES", "")

	localReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/login", nil)
	if secureCookieForRequest(localReq) {
		t.Fatal("expected localhost to default to non-secure cookie")
	}

	prodReq := httptest.NewRequest(http.MethodGet, "https://portal.example/login", nil)
	prodReq.Host = "portal.example"
	if !secureCookieForRequest(prodReq) {
		t.Fatal("expected non-localhost host to default to secure cookie")
	}
}

func TestSecureCookieForRequest_ExplicitOverride(t *testing.T) {
	os.Setenv("SECURE_COOKIES", "false")
	defer os.Unsetenv("SECURE_COOKIES")
	req := httptest.NewRequest(http.MethodGet, "https://portal.example/login", nil)
	if secureCookieForRequest(req) {
		t.Fatal("expected explicit SECURE_COOKIES=false to disable secure cookies")
	}
}
