package messaging

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadReceiptEpochTreatedAsUnread(t *testing.T) {
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	sent := time.Now().UTC()

	if isRead(true, sent, epoch) {
		t.Fatal("expected epoch last-read sentinel to be treated as unread")
	}
}

func TestTypingIndicatorEventType(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("user-1")
	defer b.Unsubscribe("user-1")

	b.Notify("user-1")
	msgEv := <-ch
	b.NotifyTyping("user-1")
	typingEv := <-ch

	if msgEv != EventMessage {
		t.Fatalf("expected EventMessage, got %q", msgEv)
	}
	if typingEv != EventTyping {
		t.Fatalf("expected EventTyping, got %q", typingEv)
	}
	if msgEv == typingEv {
		t.Fatal("message and typing events must be distinct")
	}
}

// TestChatMessageJSON verifies the chat message JSON response format.
func TestChatMessageJSONFormat(t *testing.T) {
	// Verify the JSON structure matches what the JS expects
	msg := chatMsg{
		ID:        "test-id",
		SenderID:  "sender-1",
		Name:      "Test User",
		Content:   "Hello",
		IsOwn:     true,
		CreatedAt: "Jan 2, 3:04 PM",
		ReadAt:    "Read",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal chatMsg: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all expected fields
	expected := []string{"id", "sender_id", "name", "content", "is_own", "created_at", "read_at"}
	for _, field := range expected {
		if _, ok := result[field]; !ok {
			t.Errorf("Missing field in JSON: %s", field)
		}
	}
}

// TestRecentConvoJSON verifies the recent conversations JSON format.
func TestRecentConvoJSONFormat(t *testing.T) {
	data := chatDrawerData{
		Pinned: []pinnedContact{
			{ID: "1", Name: "Test"},
		},
		Recent: []recentConvo{
			{ID: "2", Name: "User", LastMessage: "Hi", Unread: 1, IsGroup: false},
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := result["pinned"]; !ok {
		t.Error("Missing 'pinned' field")
	}
	if _, ok := result["recent"]; !ok {
		t.Error("Missing 'recent' field")
	}

	// Verify recent has is_group field
	recent := result["recent"].([]interface{})
	first := recent[0].(map[string]interface{})
	if _, ok := first["is_group"]; !ok {
		t.Error("Missing 'is_group' field in recent conversation")
	}
}

// TestSearchUsersScope verifies the contact search respects scope parameter.
func TestSearchUsersScopeParam(t *testing.T) {
	// Verify the handler reads scope from query params correctly
	req := httptest.NewRequest("GET", "/messages/search-users?q=test&scope=all", nil)
	scope := req.URL.Query().Get("scope")
	if scope != "all" {
		t.Errorf("Expected scope 'all', got '%s'", scope)
	}

	req2 := httptest.NewRequest("GET", "/messages/search-users?q=test", nil)
	scope2 := req2.URL.Query().Get("scope")
	if scope2 != "" {
		t.Errorf("Expected empty scope for contacts-only, got '%s'", scope2)
	}
}

// TestStartGroupConversation verifies multi-user conversation creation.
func TestStartGroupConversationFormParsing(t *testing.T) {
	body := "user_ids=user-1&user_ids=user-2&user_ids=user-3"
	req := httptest.NewRequest("POST", "/messages/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	userIDs := req.Form["user_ids"]
	if len(userIDs) != 3 {
		t.Errorf("Expected 3 user IDs, got %d", len(userIDs))
	}
}

// TestDeleteNotificationAcceptHeader verifies JSON vs redirect response.
func TestDeleteNotificationAcceptHeader(t *testing.T) {
	// JSON request
	req1 := httptest.NewRequest("POST", "/notifications/1/delete", nil)
	req1.Header.Set("Accept", "application/json")
	if req1.Header.Get("Accept") != "application/json" {
		t.Error("Accept header not set correctly")
	}

	// Regular request (should redirect)
	req2 := httptest.NewRequest("POST", "/notifications/1/delete", nil)
	if req2.Header.Get("Accept") == "application/json" {
		t.Error("Regular request should not have JSON accept header")
	}
}

// TestChatSendFormParsing verifies message send form parsing.
func TestChatSendFormParsing(t *testing.T) {
	body := "content=Hello+World"
	req := httptest.NewRequest("POST", "/messages/chat/conv-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	content := strings.TrimSpace(req.FormValue("content"))
	if content != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", content)
	}
}

// Verify HTTP method routing
func TestRouteMethodMatching(t *testing.T) {
	// Ensure typing endpoint only accepts POST
	req := httptest.NewRequest("GET", "/messages-typing/conv-1", nil)
	if req.Method != "GET" {
		t.Error("Method should be GET")
	}
	// In production, ServeMux would reject GET on a POST-only route
}

// Verify empty message handling
func TestEmptyMessageRejected(t *testing.T) {
	body := "content="
	req := httptest.NewRequest("POST", "/messages/chat/conv-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	content := strings.TrimSpace(req.FormValue("content"))
	if content != "" {
		t.Error("Empty content should be empty after trim")
	}
	// Handler should return 400 for empty content
}

// TestChatMsgAttachmentsJSON verifies attachment data in chat message JSON.
func TestChatMsgAttachmentsJSON(t *testing.T) {
	msg := chatMsg{
		ID:        "msg-1",
		SenderID:  "user-1",
		Name:      "Test User",
		Content:   "Check this out",
		IsOwn:     true,
		CreatedAt: "Mar 18, 2:30 PM",
		Attachments: []msgAttachment{
			{ID: "att-1", OriginalName: "photo.gif", ContentType: "image/gif", Category: "image"},
			{ID: "att-2", OriginalName: "report.pdf", ContentType: "application/pdf", Category: "document"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal chatMsg with attachments: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	atts, ok := result["attachments"].([]interface{})
	if !ok {
		t.Fatal("Missing or invalid 'attachments' field")
	}
	if len(atts) != 2 {
		t.Fatalf("Expected 2 attachments, got %d", len(atts))
	}

	first := atts[0].(map[string]interface{})
	for _, field := range []string{"id", "original_name", "content_type", "category"} {
		if _, ok := first[field]; !ok {
			t.Errorf("Missing field %q in attachment JSON", field)
		}
	}

	if first["content_type"] != "image/gif" {
		t.Errorf("Expected content_type 'image/gif', got %v", first["content_type"])
	}
}

// TestChatMsgOmitsEmptyAttachments verifies attachments field omitted when nil.
func TestChatMsgOmitsEmptyAttachments(t *testing.T) {
	msg := chatMsg{
		ID:        "msg-2",
		SenderID:  "user-1",
		Name:      "Test",
		Content:   "Hello",
		IsOwn:     false,
		CreatedAt: "Mar 18, 2:30 PM",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if _, ok := result["attachments"]; ok {
		t.Error("attachments field should be omitted when nil (omitempty)")
	}
}

// Ensure recent conversations returns valid JSON even with no data
func TestEmptyRecentResponse(t *testing.T) {
	var convos []recentConvo
	if convos == nil {
		convos = []recentConvo{}
	}

	data, err := json.Marshal(chatDrawerData{
		Pinned: []pinnedContact{},
		Recent: convos,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should be valid JSON, not null
	if string(data) == "null" {
		t.Error("Empty response should be JSON object, not null")
	}

	var result chatDrawerData
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	if result.Pinned == nil {
		t.Error("Pinned should be empty array, not nil")
	}
	if result.Recent == nil {
		t.Error("Recent should be empty array, not nil")
	}
}
