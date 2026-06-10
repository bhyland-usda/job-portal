package feed

import (
	"context"
	"database/sql"
	"html/template"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
)

// testPages builds the templates the feed handlers render, mirroring the
// production parseTemplate wiring (layouts + the page template).
func testPages() map[string]*template.Template {
	tmplDir := "../../templates"
	mk := func(name string) *template.Template {
		return template.Must(template.ParseFiles(
			tmplDir+"/layouts/base.html",
			tmplDir+"/layouts/navbar.html",
			tmplDir+"/"+name,
		))
	}
	return map[string]*template.Template{
		"feed.html":     mk("feed/feed.html"),
		"drafts.html":   mk("feed/drafts.html"),
		"trending.html": mk("feed/trending.html"),
	}
}

// TestHashtagRegex verifies hashtag extraction from post content.
func TestHashtagRegex(t *testing.T) {
	re := regexp.MustCompile(`#(\w+)`)

	tests := []struct {
		input    string
		expected []string
	}{
		{"Hello #world", []string{"world"}},
		{"#Go and #Rust are great", []string{"Go", "Rust"}},
		{"No hashtags here", nil},
		{"#GIS #DataScience #Python", []string{"GIS", "DataScience", "Python"}},
		{"Email test@example.com", nil},           // @ is not #
		{"#123numeric", []string{"123numeric"}},   // numbers allowed
		{"#under_score", []string{"under_score"}}, // underscore allowed
		{"Multiple ##double", []string{"double"}}, // double # still captures
		{"End of line #tag", []string{"tag"}},     // end of string
		{"#a", []string{"a"}},                     // single char
	}

	for _, tt := range tests {
		matches := re.FindAllStringSubmatch(tt.input, -1)
		var got []string
		for _, m := range matches {
			got = append(got, m[1])
		}
		if len(got) != len(tt.expected) {
			t.Errorf("Input %q: expected %v, got %v", tt.input, tt.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("Input %q: tag %d expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

// TestMentionRegex verifies @mention extraction from post content.
func TestMentionRegex(t *testing.T) {
	re := regexp.MustCompile(`@(\w+\s\w+)`)

	tests := []struct {
		input    string
		expected []string
	}{
		{"Hey @Bryan Hyland check this out", []string{"Bryan Hyland"}},
		{"@Clark Kent and @Diana Prince", []string{"Clark Kent", "Diana Prince"}},
		{"No mentions here", nil},
		{"Email user@example com", []string{"example com"}}, // false positive - known limitation
		{"@Single", nil},                                    // single word - no match
	}

	for _, tt := range tests {
		matches := re.FindAllStringSubmatch(tt.input, -1)
		var got []string
		for _, m := range matches {
			got = append(got, m[1])
		}
		if len(got) != len(tt.expected) {
			t.Errorf("Input %q: expected %d matches, got %d (%v)", tt.input, len(tt.expected), len(got), got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("Input %q: match %d expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

// TestReactionTypes verifies valid reaction types.
func TestReactionTypes(t *testing.T) {
	validReactions := map[string]bool{"like": true, "celebrate": true, "insightful": true, "curious": true}

	tests := []struct {
		input string
		valid bool
	}{
		{"like", true},
		{"celebrate", true},
		{"insightful", true},
		{"curious", true},
		{"love", false},
		{"", false},
		{"LIKE", false}, // case sensitive
	}

	for _, tt := range tests {
		if validReactions[tt.input] != tt.valid {
			t.Errorf("Reaction %q: expected valid=%v, got valid=%v", tt.input, tt.valid, validReactions[tt.input])
		}
	}
}

// TestPostContentTrim verifies empty post rejection.
func TestPostContentTrim(t *testing.T) {
	tests := []struct {
		input string
		empty bool
	}{
		{"Hello world", false},
		{"", true},
		{"   ", true},
		{"\t\n", true},
		{"  valid  ", false},
	}

	for _, tt := range tests {
		content := strings.TrimSpace(tt.input)
		if (content == "") != tt.empty {
			t.Errorf("Input %q: expected empty=%v, got empty=%v", tt.input, tt.empty, content == "")
		}
	}
}

// TestSharePostCreatesVisibleFeedPost verifies a share now creates both a
// shared_posts row and a normal feed post so the share appears in timelines.
func TestSharePostCreatesVisibleFeedPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT content FROM posts WHERE id = \$1`).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow("original post body"))

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO shared_posts \(user_id, original_post_id, commentary\) VALUES \(\$1, \$2, \$3\)`).
		WithArgs("user-7", "post-1", "Check this out").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO posts \(user_id, content, scheduled_at\) VALUES \(\$1, \$2, \$3\)`).
		WithArgs("user-7", "Check this out\n\n---\noriginal post body", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// notifyPostAuthor lookup.
	mock.ExpectQuery(`SELECT user_id FROM posts WHERE id = \$1`).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("author-1"))
	mock.ExpectQuery(`SELECT first_name, last_name FROM users WHERE id = \$1`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name"}).AddRow("Ada", "Lovelace"))
	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("author-1", "feed").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO notifications \(user_id, sender_id, type, message\)`).
		WithArgs("author-1", "user-7", "feed", "Ada Lovelace shared your post").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := &Handler{db: db, notif: notification.NewHandler(db, nil), broker: NewBroker()}

	form := url.Values{}
	form.Set("commentary", "Check this out")
	req := httptest.NewRequest("POST", "/feed/post-1/share", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "post-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.sharePost(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/feed" {
		t.Fatalf("expected redirect to /feed, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedTemplateRendersAttachments(t *testing.T) {
	b, err := os.ReadFile("../../templates/feed/feed.html")
	if err != nil {
		t.Fatalf("read feed template: %v", err)
	}
	s := string(b)

	for _, want := range []string{"{{if .Attachments}}", "post-attachments", "/feed/attachment/{{.ID}}"} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected feed template to contain %q", want)
		}
	}
}

func TestBookmarkButtonsPresentAcrossPrimaryViews(t *testing.T) {
	templates := []string{
		"../../templates/feed/feed.html",
		"../../templates/opportunity/view.html",
		"../../templates/article/view.html",
	}
	for _, path := range templates {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read template %s: %v", path, err)
		}
		s := string(b)
		if !strings.Contains(s, "bookmark-btn") {
			t.Fatalf("expected bookmark control in %s", path)
		}
	}
}

// TestGetSocialFeedExcludesFutureScheduled verifies the social feed query filters
// out posts whose scheduled_at is in the future, while keeping NULL/past ones.
func TestGetSocialFeedExcludesFutureScheduled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// The query must constrain on scheduled_at so future posts are hidden.
	// We return only an immediate (NULL scheduled_at) post and a past one.
	mock.ExpectQuery(`scheduled_at IS NULL OR p\.scheduled_at <= now\(\)`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "first_name", "last_name", "avatar_url", "headline",
			"content", "created_at",
			"like_count", "comment_count", "liked_by_user", "reaction_type", "bookmarked",
		}).
			AddRow("post-immediate", "user-7", "Ada", "Lovelace", "", "Engineer",
				"published now", created, 0, 0, false, "", false).
			AddRow("post-past", "user-7", "Ada", "Lovelace", "", "Engineer",
				"published earlier", created, 0, 0, false, "", false))

	// Attachment lookups per post (no attachments).
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`FROM post_attachments WHERE post_id = \$1`).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "original_name", "content_type", "file_size", "file_category",
			}))
	}

	h := &Handler{db: db}

	req := httptest.NewRequest("GET", "/feed", nil)
	posts, err := h.getSocialFeed(req, "user-7", "")
	if err != nil {
		t.Fatalf("getSocialFeed returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 visible posts, got %d", len(posts))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations (query likely missing scheduled_at filter): %v", err)
	}
}

// TestGetSocialFeedIncludesHighlightedPost verifies a highlighted post deep-link
// is loaded when it is not already in the default feed slice.
func TestGetSocialFeedIncludesHighlightedPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	// Base feed is empty (e.g. highlighted post older than top 50 or from a
	// connected user outside the default window).
	mock.ExpectQuery(`FROM posts p`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "first_name", "last_name", "avatar_url", "headline",
			"content", "created_at",
			"like_count", "comment_count", "liked_by_user", "reaction_type", "bookmarked",
		}))

	// Highlighted post query should load exactly one authorized post.
	mock.ExpectQuery(`WHERE p.id = \$2`).
		WithArgs("user-7", "post-42").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "first_name", "last_name", "avatar_url", "headline",
			"content", "created_at",
			"like_count", "comment_count", "liked_by_user", "reaction_type", "bookmarked",
		}).AddRow(
			"post-42", "author-2", "Grace", "Hopper", "", "Engineer",
			"older post", created,
			0, 0, false, "", true,
		))

	// Attachment query for the highlighted post.
	mock.ExpectQuery(`FROM post_attachments WHERE post_id = \$1`).
		WithArgs("post-42").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "original_name", "content_type", "file_size", "file_category",
		}))

	h := &Handler{db: db}
	req := httptest.NewRequest("GET", "/feed?tab=social&highlight=post-42", nil)

	posts, err := h.getSocialFeed(req, "user-7", "post-42")
	if err != nil {
		t.Fatalf("getSocialFeed returned error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post (highlight only), got %d", len(posts))
	}
	if posts[0].ID != "post-42" {
		t.Fatalf("expected highlighted post id post-42, got %s", posts[0].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCreatePostStoresFutureScheduledAt verifies that supplying a future
// schedule_at form value persists it on insert.
func TestCreatePostStoresFutureScheduledAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	future := time.Now().Add(48 * time.Hour).UTC().Format("2006-01-02T15:04")

	// Insert must carry a non-NULL scheduled_at argument.
	mock.ExpectQuery(`INSERT INTO posts \(user_id, content, scheduled_at\) VALUES \(\$1, \$2, \$3\) RETURNING id`).
		WithArgs("user-7", "scheduled hello", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("post-1"))

	h := &Handler{db: db, broker: NewBroker()}

	form := url.Values{}
	form.Set("content", "scheduled hello")
	form.Set("scheduled_at", future)

	req := httptest.NewRequest("POST", "/feed", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.createPost(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations (scheduled_at not stored on insert): %v", err)
	}
}

// TestCreatePostImmediateScheduledAtNull verifies that with no schedule input the
// post is inserted with a NULL scheduled_at (published immediately).
func TestCreatePostImmediateScheduledAtNull(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO posts \(user_id, content, scheduled_at\) VALUES \(\$1, \$2, \$3\) RETURNING id`).
		WithArgs("user-7", "hello now", nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("post-1"))

	h := &Handler{db: db, broker: NewBroker()}

	form := url.Values{}
	form.Set("content", "hello now")

	req := httptest.NewRequest("POST", "/feed", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.createPost(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowTrendingRanksRecentHashtags verifies the trending handler windows on
// the last 7 days and renders hashtags ordered by recent activity, each with its
// post count and a link to the hashtag feed.
func TestShowTrendingRanksRecentHashtags(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// The query must window on recent post activity and rank descending. We
	// return three already-ranked rows (name, post_count, score).
	mock.ExpectQuery(`7 days`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "post_count", "score"}).
			AddRow("gis", 12, 40).
			AddRow("datascience", 8, 25).
			AddRow("python", 5, 11))

	h := &Handler{db: db, pages: testPages()}

	req := httptest.NewRequest("GET", "/feed/trending", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.showTrending(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	// Each ranked tag must render with its name, count, and hashtag-feed link.
	for _, want := range []string{
		"#gis", "12 post",
		"#datascience", "8 post",
		"#python", "5 post",
		`/feed/hashtag/gis`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got:\n%s", want, body)
		}
	}

	// Ranking order must be preserved: gis before datascience before python.
	gis := strings.Index(body, "#gis")
	ds := strings.Index(body, "#datascience")
	py := strings.Index(body, "#python")
	if !(gis < ds && ds < py) {
		t.Errorf("expected ranked order gis < datascience < python, got positions %d, %d, %d", gis, ds, py)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations (trending query likely missing 7-day window): %v", err)
	}
}

func TestServeAttachmentUnauthorizedReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM post_attachments pa`).
		WithArgs("att-1", "viewer-1").
		WillReturnError(sql.ErrNoRows)

	h := &Handler{db: db}
	req := httptest.NewRequest("GET", "/feed/attachment/att-1", nil)
	req.SetPathValue("id", "att-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "viewer-1"))
	rec := httptest.NewRecorder()

	h.serveAttachment(rec, req)

	if rec.Code != 404 {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestShowTrendingEmptyState verifies the handler degrades gracefully to the
// empty-state message when there is no recent hashtag activity.
func TestShowTrendingEmptyState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`7 days`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "post_count", "score"}))

	h := &Handler{db: db, pages: testPages()}

	req := httptest.NewRequest("GET", "/feed/trending", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.showTrending(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "No trending topics") {
		t.Errorf("expected empty-state message, got:\n%s", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestHandleMentionRegex verifies the handle-based @mention regex captures the
// email local-part style tokens used for tagging (e.g. @clark.kent).
func TestHandleMentionRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Hey @clark.kent welcome aboard", []string{"clark.kent"}},
		{"cc @diana.prince and @bruce-wayne", []string{"diana.prince", "bruce-wayne"}},
		{"No mentions here", nil},
		{"under_score @j_doe ok", []string{"j_doe"}},
		{"numbers @agent007 fine", []string{"agent007"}},
		{"email like clark.kent@usda.gov", []string{"usda.gov"}}, // matches text after @ (known limitation)
	}

	for _, tt := range tests {
		matches := handleMentionRegex.FindAllStringSubmatch(tt.input, -1)
		var got []string
		for _, m := range matches {
			got = append(got, m[1])
		}
		if len(got) != len(tt.expected) {
			t.Errorf("Input %q: expected %v, got %v", tt.input, tt.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("Input %q: match %d expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

// TestCreatePostNotifiesMentionedUser verifies that a post containing a valid
// @handle resolves the handle to a user and creates a "mention" notification.
func TestCreatePostNotifiesMentionedUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Insert the post.
	mock.ExpectQuery(`INSERT INTO posts \(user_id, content, scheduled_at\) VALUES \(\$1, \$2, \$3\) RETURNING id`).
		WithArgs("author-1", "hey @clark.kent welcome", nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("post-1"))

	// Resolve the handle to a user id via email local-part.
	mock.ExpectQuery(`SELECT id FROM users WHERE lower\(split_part\(email,'@',1\)\) = \$1`).
		WithArgs("clark.kent").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("clark-9"))

	// Look up the author's name for the notification message.
	mock.ExpectQuery(`SELECT first_name, last_name FROM users WHERE id = \$1`).
		WithArgs("author-1").
		WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name"}).AddRow("Lois", "Lane"))

	// CreateNotification checks the recipient's notification preference first.
	mock.ExpectQuery(`SELECT enabled FROM notification_preferences`).
		WithArgs("clark-9", "mention").
		WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))

	// The mention must produce a notification of type "mention" to the resolved user.
	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs("clark-9", "author-1", "mention", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	notif := notification.NewHandler(db, nil)
	h := &Handler{db: db, broker: NewBroker(), notif: notif}

	form := url.Values{}
	form.Set("content", "hey @clark.kent welcome")

	req := httptest.NewRequest("POST", "/feed", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "author-1"))
	rec := httptest.NewRecorder()

	h.createPost(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations (mention notification not created): %v", err)
	}
}

// TestShowDraftsListsDraftsAndScheduledPosts verifies drafts page now includes
// both saved drafts and future scheduled social posts.
func TestShowDraftsListsDraftsAndScheduledPosts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	scheduled := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM post_drafts`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "created_at"}).
			AddRow("draft-1", "draft text", created))

	mock.ExpectQuery(`scheduled_at > now\(\)`).
		WithArgs("user-7").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "created_at", "scheduled_at"}).
			AddRow("post-future", "see you in june", created, scheduled))

	h := &Handler{db: db, pages: testPages()}

	req := httptest.NewRequest("GET", "/feed/drafts", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.showDrafts(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "draft text") {
		t.Errorf("expected draft content in body, got:\n%s", body)
	}
	if !strings.Contains(body, "see you in june") {
		t.Errorf("expected scheduled post content in body, got:\n%s", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRedirectScheduledToDrafts(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/feed/scheduled", nil)
	rec := httptest.NewRecorder()

	h.redirectScheduledToDrafts(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/feed/drafts" {
		t.Fatalf("expected redirect to /feed/drafts, got %q", loc)
	}
}

func TestScheduleDraftPostStoresFutureScheduledPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`INSERT INTO posts \(user_id, content, scheduled_at\) VALUES \(\$1, \$2, \$3\)`).
		WithArgs("user-7", "scheduled from drafts", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &Handler{db: db}
	future := time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")

	form := url.Values{}
	form.Set("content", "scheduled from drafts")
	form.Set("scheduled_at", future)

	req := httptest.NewRequest("POST", "/feed/drafts/schedule", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.scheduleDraftPost(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/feed/drafts" {
		t.Fatalf("expected redirect to /feed/drafts, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSaveDraftStoresRichMarkdownWhenRequested(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rich := "**Bold intro**"
	mock.ExpectExec(`INSERT INTO post_drafts \(user_id, content\) VALUES \(\$1, \$2\)`).
		WithArgs("user-7", richMarkdownPrefix+rich).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &Handler{db: db}
	form := url.Values{}
	form.Set("content", rich)
	form.Set("content_format", "rich_markdown")

	req := httptest.NewRequest("POST", "/feed/drafts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.saveDraft(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRenderContentRichMarkdownToHTML(t *testing.T) {
	h := &Handler{}
	html := h.renderContent(context.Background(), richMarkdownPrefix+"**Bold**\n\n- One\n- Two")
	rendered := string(html)

	if !strings.Contains(rendered, "<strong>Bold</strong>") {
		t.Fatalf("expected bold markdown rendered to strong tag, got %q", rendered)
	}
	if !strings.Contains(rendered, "<ul>") || !strings.Contains(rendered, "<li>One</li>") {
		t.Fatalf("expected list markdown rendered to ul/li tags, got %q", rendered)
	}
}

func TestPublishDraftPreservesRichMarkdownContent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	richStored := richMarkdownPrefix + "**Release note**\n\n- line item"

	mock.ExpectQuery(`SELECT content FROM post_drafts WHERE id = \$1 AND user_id = \$2`).
		WithArgs("draft-1", "user-7").
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(richStored))

	mock.ExpectExec(`INSERT INTO posts \(user_id, content\) VALUES \(\$1, \$2\)`).
		WithArgs("user-7", richStored).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`DELETE FROM post_drafts WHERE id = \$1`).
		WithArgs("draft-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &Handler{db: db, broker: NewBroker()}
	req := httptest.NewRequest("POST", "/feed/drafts/draft-1/publish", nil)
	req.SetPathValue("id", "draft-1")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-7"))
	rec := httptest.NewRecorder()

	h.publishDraft(rec, req)

	if rec.Code != 303 {
		t.Fatalf("expected 303 redirect, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/feed" {
		t.Fatalf("expected redirect to /feed, got %q", loc)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
