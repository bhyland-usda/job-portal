package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve source path: runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

type fixtureServer struct {
	baseURL string
	client  *http.Client
	cancel  context.CancelFunc
	cmd     *exec.Cmd
}

func startFixtureServer(t *testing.T) *fixtureServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	workflowFixtures = newFixtureState()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/bots/testserver")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	var startupOutput strings.Builder
	cmd.Stdout = &startupOutput
	cmd.Stderr = &startupOutput

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start server: %v", err)
	}

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if time.Now().After(deadline) {
			cancel()
			_ = cmd.Wait()
			t.Fatalf("server did not start in time\n%s", startupOutput.String())
		}
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		cancel()
		_ = cmd.Wait()
		t.Fatalf("cookie jar: %v", err)
	}

	return &fixtureServer{
		baseURL: baseURL,
		client:  &http.Client{Jar: jar, Timeout: 10 * time.Second},
		cancel:  cancel,
		cmd:     cmd,
	}
}

func (s *fixtureServer) close() {
	s.cancel()
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *fixtureServer) get(t *testing.T, path string) (*http.Response, string) {
	t.Helper()

	resp, err := s.client.Get(s.baseURL + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

func (s *fixtureServer) postForm(t *testing.T, path string, values url.Values) (*http.Response, string) {
	t.Helper()

	resp, err := s.client.PostForm(s.baseURL+path, values)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	_ = resp.Body.Close()
	return resp, string(body)
}

func TestNotificationsMutation(t *testing.T) {
	server := startFixtureServer(t)
	defer server.close()

	_, body := server.get(t, "/notifications/count?role=employee")
	var countResp map[string]int
	if err := json.Unmarshal([]byte(body), &countResp); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	if countResp["count"] != 2 {
		t.Fatalf("expected unread count 2, got %d", countResp["count"])
	}

	server.postForm(t, "/notifications/notif-1/read?role=employee", url.Values{})
	_, body = server.get(t, "/notifications/count?role=employee")
	if err := json.Unmarshal([]byte(body), &countResp); err != nil {
		t.Fatalf("unmarshal count after read: %v", err)
	}
	if countResp["count"] != 1 {
		t.Fatalf("expected unread count 1 after read, got %d", countResp["count"])
	}

	server.postForm(t, "/notifications/read-all?role=employee", url.Values{})
	_, body = server.get(t, "/notifications/count?role=employee")
	if err := json.Unmarshal([]byte(body), &countResp); err != nil {
		t.Fatalf("unmarshal count after read-all: %v", err)
	}
	if countResp["count"] != 0 {
		t.Fatalf("expected unread count 0 after read-all, got %d", countResp["count"])
	}

	server.postForm(t, "/notifications/notif-3/delete?role=employee", url.Values{})
	_, body = server.get(t, "/notifications/recent?role=employee")
	var notifications []map[string]any
	if err := json.Unmarshal([]byte(body), &notifications); err != nil {
		t.Fatalf("unmarshal notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("expected 2 notifications after delete, got %d", len(notifications))
	}
}

func TestMessagingWorkflow(t *testing.T) {
	server := startFixtureServer(t)
	defer server.close()

	_, body := server.get(t, "/messages/search-users?q=jordan&role=employee")
	if !strings.Contains(body, "Jordan Kim") {
		t.Fatalf("expected search results to include Jordan Kim, got %s", body)
	}

	_, body = server.postForm(t, "/messages/start?role=employee", url.Values{
		"user_ids": {"mentor-2"},
		"name":     {"Sprint Sync"},
	})
	var startResp map[string]string
	if err := json.Unmarshal([]byte(body), &startResp); err != nil {
		t.Fatalf("unmarshal conversation start: %v", err)
	}
	conversationID := startResp["id"]
	if conversationID == "" {
		t.Fatalf("expected conversation id, got %q", conversationID)
	}

	server.postForm(t, "/messages/chat/"+conversationID+"?role=employee", url.Values{
		"content": {"Hello team"},
	})

	resp, body := server.get(t, "/messages/chat/"+conversationID+"?role=employee")
	if got := resp.Header.Get("X-Conversation-Name"); got != "Sprint Sync" {
		t.Fatalf("expected conversation header Sprint Sync, got %q", got)
	}
	if !strings.Contains(body, "Hello team") {
		t.Fatalf("expected transcript to include new message, got %s", body)
	}

	server.postForm(t, "/messages/chat/"+conversationID+"/members?role=employee", url.Values{
		"user_id": {"coworker-1"},
	})

	_, body = server.get(t, "/messages/recent?role=employee")
	if !strings.Contains(body, "Sprint Sync") {
		t.Fatalf("expected recent conversations to include Sprint Sync, got %s", body)
	}
}

func TestWorkflowPagesMutateState(t *testing.T) {
	server := startFixtureServer(t)
	defer server.close()

	_, body := server.get(t, "/postings/posting-1?role=employee")
	if !strings.Contains(body, "Save Posting") {
		t.Fatalf("expected posting page to show save action")
	}

	server.postForm(t, "/bookmarks/add?role=employee", url.Values{
		"target_type": {"posting"},
		"target_id":   {"posting-1"},
	})
	_, body = server.get(t, "/postings/posting-1?role=employee")
	if !strings.Contains(body, "Saved") {
		t.Fatalf("expected posting page to show saved state")
	}
	_, body = server.get(t, "/bookmarks?tab=posting&role=employee")
	if !strings.Contains(body, "Temporary detail opportunity") {
		t.Fatalf("expected bookmarks page to include saved posting")
	}

	_, body = server.get(t, "/groups/group-1?role=employee")
	if !strings.Contains(body, "Leave Group") {
		t.Fatalf("expected initial group membership")
	}
	server.postForm(t, "/groups/group-1/leave?role=employee", url.Values{})
	_, body = server.get(t, "/groups/group-1?role=employee")
	if !strings.Contains(body, "Join Group") {
		t.Fatalf("expected group to show join action after leaving")
	}
	server.postForm(t, "/groups/group-1/join?role=employee", url.Values{})
	server.postForm(t, "/groups/group-1/post?role=employee", url.Values{
		"content": {"Added from test"},
	})
	_, body = server.get(t, "/groups/group-1?role=employee")
	if !strings.Contains(body, "Added from test") {
		t.Fatalf("expected group discussion to include new post")
	}

	server.postForm(t, "/workspaces/workspace-1/notes?role=employee", url.Values{
		"body": {"Need to review the launch checklist."},
	})
	server.postForm(t, "/workspaces/workspace-1/members?role=employee", url.Values{
		"user_id": {"admin-1"},
	})
	_, body = server.get(t, "/workspaces/workspace-1?role=employee")
	if !strings.Contains(body, "Need to review the launch checklist.") {
		t.Fatalf("expected workspace note to be visible")
	}
	if !strings.Contains(body, "Sam Patel") {
		t.Fatalf("expected workspace member add to be visible")
	}

	_, body = server.get(t, "/polls?role=employee")
	if !strings.Contains(body, "Vote") {
		t.Fatalf("expected poll page to show vote form")
	}
	server.postForm(t, "/polls/poll-1/vote?role=employee", url.Values{
		"option": {"opt-2"},
	})
	_, body = server.get(t, "/polls?role=employee")
	if !strings.Contains(body, "6 votes") || !strings.Contains(body, "3 (50%)") {
		t.Fatalf("expected poll results to reflect vote, got %s", body)
	}
}

func TestAdminAndPolicyFlows(t *testing.T) {
	server := startFixtureServer(t)
	defer server.close()

	_, body := server.get(t, "/profile/coworker-2?role=employee")
	if !strings.Contains(body, "Accept") {
		t.Fatalf("expected pending connection on coworker-2 profile")
	}
	server.postForm(t, "/connections/accept/coworker-2?role=employee", url.Values{})
	_, body = server.get(t, "/profile/coworker-2?role=employee")
	if !strings.Contains(body, "Connected") {
		t.Fatalf("expected connection accept to update profile state")
	}

	server.postForm(t, "/aup/accept?role=employee", url.Values{})
	_, body = server.get(t, "/aup?role=employee")
	if !strings.Contains(body, "You have accepted this policy.") {
		t.Fatalf("expected AUP accepted state")
	}

	_, body = server.postForm(t, "/messages/new/mentor-2?role=employee", url.Values{})
	var newResp map[string]string
	if err := json.Unmarshal([]byte(body), &newResp); err != nil {
		t.Fatalf("unmarshal direct conversation: %v", err)
	}
	server.postForm(t, "/messages/chat/"+newResp["id"]+"?role=employee", url.Values{
		"content": {"FOIA sync note"},
	})

	_, body = server.get(t, "/admin/foia/search?role=admin&type=message&user=Riley")
	if !strings.Contains(body, "FOIA sync note") {
		t.Fatalf("expected FOIA search to include latest message")
	}

	_, body = server.get(t, "/admin/foia/export?role=admin&type=message&user=Riley")
	if !strings.Contains(body, "FOIA sync note") || !strings.Contains(body, "\"status\":\"ready\"") {
		t.Fatalf("expected FOIA export to remain coherent, got %s", body)
	}

	_, body = server.get(t, "/admin/moderation?role=admin")
	if !strings.Contains(body, "Contains outdated staffing numbers.") {
		t.Fatalf("expected moderation queue to show report")
	}
	server.postForm(t, "/admin/moderation/report-1/resolve?role=admin", url.Values{
		"action": {"reviewed"},
	})
	_, body = server.get(t, "/admin/moderation?role=admin")
	if !strings.Contains(body, "The queue is clear.") {
		t.Fatalf("expected moderation queue to clear after resolve")
	}
}
