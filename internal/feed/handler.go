package feed

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/attachment"
	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
	"github.com/bhyland-usda/job-portal/internal/opportunity"
	"github.com/bhyland-usda/job-portal/internal/semantic"
	"github.com/yuin/goldmark"
)

var hashtagRegex = regexp.MustCompile(`#(\w+)`)
var richMarkdownPrefix = "[[RICH_MARKDOWN]]\n"

// handleMentionRegex matches @handle tokens where a handle is an email
// local-part (e.g. @clark.kent for clark.kent@usda.gov).
var handleMentionRegex = regexp.MustCompile(`@([a-z0-9._-]+)`)

type Handler struct {
	db     *sql.DB
	pages  map[string]*template.Template
	notif  *notification.Handler
	broker *Broker
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, notif *notification.Handler) *Handler {
	return &Handler{db: db, pages: pages, notif: notif, broker: NewBroker()}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /feed", requireAuth(http.HandlerFunc(h.showFeed)))
	mux.Handle("GET /feed/events", requireAuth(http.HandlerFunc(h.streamFeed)))
	mux.Handle("POST /feed", requireAuth(http.HandlerFunc(h.createPost)))
	mux.Handle("POST /feed/{id}/like", requireAuth(http.HandlerFunc(h.toggleLike)))
	mux.Handle("POST /feed/{id}/comment", requireAuth(http.HandlerFunc(h.addComment)))
	mux.Handle("POST /feed/{id}/delete", requireAuth(http.HandlerFunc(h.deletePost)))
	mux.Handle("POST /feed/{id}/share", requireAuth(http.HandlerFunc(h.sharePost)))
	mux.Handle("GET /feed/hashtag/{tag}", requireAuth(http.HandlerFunc(h.showHashtagFeed)))
	mux.Handle("GET /feed/attachment/{id}", requireAuth(http.HandlerFunc(h.serveAttachment)))
	mux.Handle("GET /feed/trending", requireAuth(http.HandlerFunc(h.showTrending)))
	mux.Handle("GET /feed/scheduled", requireAuth(http.HandlerFunc(h.redirectScheduledToDrafts)))
	mux.Handle("GET /feed/drafts", requireAuth(http.HandlerFunc(h.showDrafts)))
	mux.Handle("POST /feed/drafts", requireAuth(http.HandlerFunc(h.saveDraft)))
	mux.Handle("POST /feed/drafts/schedule", requireAuth(http.HandlerFunc(h.scheduleDraftPost)))
	mux.Handle("POST /feed/drafts/{id}/publish", requireAuth(http.HandlerFunc(h.publishDraft)))
	mux.Handle("POST /feed/drafts/{id}/delete", requireAuth(http.HandlerFunc(h.deleteDraft)))
}

type Post struct {
	ID              string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	AuthorAvatarURL string
	AuthorHeadline  string
	Content         string
	CreatedAt       time.Time
	LikeCount       int
	CommentCount    int
	LikedByUser     bool
	UserReaction    string
	IsAuthor        bool
	Comments        []Comment
	SharedBy        string
	SharedComment   string
	OriginalPostID  string
	Attachments     []Attachment
	Bookmarked      bool
	RenderedContent template.HTML
	Preview         *LinkPreview
}

type Attachment struct {
	ID           string
	OriginalName string
	ContentType  string
	FileSize     int
	Category     string
}

type Comment struct {
	ID              string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	AuthorAvatarURL string
	Content         string
	CreatedAt       time.Time
	RenderedContent template.HTML
}

type FeedPage struct {
	middleware.BaseData
	Posts    []Post
	Tab      string
	Postings []opportunity.Posting
}

func (h *Handler) streamFeed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	respCont := http.NewResponseController(w)
	respCont.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.broker.Subscribe(userID)
	defer h.broker.Unsubscribe(userID)

	for {
		select {
		case postID, ok := <-ch:
			if !ok {
				return
			}
			if postID == "" {
				fmt.Fprint(w, "event: feed-update\ndata: refresh\n\n")
			} else {
				fmt.Fprintf(w, "event: post-update\ndata: %s\n\n", postID)
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handler) showFeed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	highlightPostID := strings.TrimSpace(r.URL.Query().Get("highlight"))
	if tab == "" {
		tab = "social"
	}

	var posts []Post
	var matchedPostings []opportunity.Posting
	var err error

	if tab == "social" {
		posts, err = h.getSocialFeed(r, userID, highlightPostID)
		if err != nil {
			slog.Error("failed to load feed", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		for i := range posts {
			comments, err := h.getComments(r, posts[i].ID)
			if err != nil {
				slog.Error("failed to load comments", "error", err)
				continue
			}
			posts[i].Comments = comments
		}
	} else if tab == "postings" {
		matchedPostings, err = opportunity.GetMatchedPostings(h.db, r.Context(), userID, semantic.EnabledForRequest(r), semantic.LocationTypePreferenceForRequest(r))
		if err != nil {
			slog.Error("failed to load postings", "error", err)
		}
	}

	data := FeedPage{
		BaseData: middleware.NewBaseData(r),
		Posts:    posts,
		Tab:      tab,
		Postings: matchedPostings,
	}

	// Record post views in background
	if tab == "social" && len(posts) > 0 {
		go func(postIDs []string, uid string) {
			for _, pid := range postIDs {
				h.db.Exec(
					`INSERT INTO post_views (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					pid, uid,
				)
			}
		}(func() []string {
			ids := make([]string, len(posts))
			for i, p := range posts {
				ids[i] = p.ID
			}
			return ids
		}(), userID)
	}

	if err := h.pages["feed.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render feed", "error", err)
	}
}

func (h *Handler) notifyPostAuthor(postID, actorID, action string) {
	var authorID string
	err := h.db.QueryRow(
		`SELECT user_id FROM posts WHERE id = $1`, postID,
	).Scan(&authorID)
	if err != nil {
		slog.Error("failed to look up post author", "error", err)
		return
	}

	if authorID == actorID {
		return
	}

	var firstName, lastName string
	err = h.db.QueryRow(
		`SELECT first_name, last_name FROM users WHERE id = $1`, actorID,
	).Scan(&firstName, &lastName)
	if err != nil {
		slog.Error("failed to look up actor name", "error", err)
		return
	}

	message := firstName + " " + lastName + " " + action
	h.notif.CreateNotification(context.Background(), authorID, actorID, "feed", message)
}

func (h *Handler) getSocialFeed(r *http.Request, userID, highlightPostID string) ([]Post, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT p.id, p.user_id, u.first_name, u.last_name, u.avatar_url, u.headline,
                        p.content, p.created_at,
                        (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id) AS like_count,
                        (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count,
                        EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_user,
                        COALESCE((SELECT reaction_type FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1), ''),
                        EXISTS(SELECT 1 FROM bookmarks b WHERE b.user_id = $1 AND b.target_type = 'post' AND b.target_id = p.id)
                 FROM posts p
                 JOIN users u ON u.id = p.user_id
                 WHERE (p.user_id = $1
                    OR p.user_id IN (
                        SELECT CASE
                            WHEN c.requester_id = $1 THEN c.addressee_id
                            ELSE c.requester_id
                        END
                        FROM connections c
                        WHERE (c.requester_id = $1 OR c.addressee_id = $1)
                          AND c.status = 'accepted'
                    )
                 )
                 AND (p.scheduled_at IS NULL OR p.scheduled_at <= now())
                 ORDER BY p.created_at DESC
                 LIMIT 50`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(
			&p.ID, &p.AuthorID, &p.AuthorFirstName, &p.AuthorLastName,
			&p.AuthorAvatarURL, &p.AuthorHeadline,
			&p.Content, &p.CreatedAt,
			&p.LikeCount, &p.CommentCount, &p.LikedByUser, &p.UserReaction,
			&p.Bookmarked,
		); err != nil {
			return nil, err
		}
		p.IsAuthor = p.AuthorID == userID
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if highlightPostID != "" {
		found := false
		for _, p := range posts {
			if p.ID == highlightPostID {
				found = true
				break
			}
		}

		if !found {
			var hp Post
			err := h.db.QueryRowContext(r.Context(),
				`SELECT p.id, p.user_id, u.first_name, u.last_name, u.avatar_url, u.headline,
                        p.content, p.created_at,
                        (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id) AS like_count,
                        (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count,
                        EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_user,
                        COALESCE((SELECT reaction_type FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1), ''),
                        EXISTS(SELECT 1 FROM bookmarks b WHERE b.user_id = $1 AND b.target_type = 'post' AND b.target_id = p.id)
                 FROM posts p
                 JOIN users u ON u.id = p.user_id
                 WHERE p.id = $2
                   AND (p.user_id = $1
                    OR p.user_id IN (
                        SELECT CASE
                            WHEN c.requester_id = $1 THEN c.addressee_id
                            ELSE c.requester_id
                        END
                        FROM connections c
                        WHERE (c.requester_id = $1 OR c.addressee_id = $1)
                          AND c.status = 'accepted'
                    ))
                   AND (p.scheduled_at IS NULL OR p.scheduled_at <= now())`,
				userID, highlightPostID,
			).Scan(
				&hp.ID, &hp.AuthorID, &hp.AuthorFirstName, &hp.AuthorLastName,
				&hp.AuthorAvatarURL, &hp.AuthorHeadline,
				&hp.Content, &hp.CreatedAt,
				&hp.LikeCount, &hp.CommentCount, &hp.LikedByUser, &hp.UserReaction,
				&hp.Bookmarked,
			)
			if err == nil {
				hp.IsAuthor = hp.AuthorID == userID
				posts = append([]Post{hp}, posts...)
			}
		}
	}

	// Load attachments for each post
	for i := range posts {
		attRows, err := h.db.QueryContext(r.Context(),
			`SELECT id, original_name, content_type, file_size, COALESCE(file_category, 'image')
                         FROM post_attachments WHERE post_id = $1`,
			posts[i].ID,
		)
		if err != nil {
			continue
		}
		for attRows.Next() {
			var a Attachment
			if err := attRows.Scan(&a.ID, &a.OriginalName, &a.ContentType, &a.FileSize, &a.Category); err == nil {
				posts[i].Attachments = append(posts[i].Attachments, a)
			}
		}
		attRows.Close()
	}

	// Linkify content (@mentions + URLs) and attach any cached link preview.
	for i := range posts {
		posts[i].RenderedContent = h.renderContent(r.Context(), posts[i].Content)
		if firstURL := extractFirstURL(contentForProcessing(posts[i].Content)); firstURL != "" {
			posts[i].Preview = loadPreview(r.Context(), h.db, firstURL)
		}
	}

	return posts, nil
}

func (h *Handler) getComments(r *http.Request, postID string) ([]Comment, error) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT c.id, c.user_id, u.first_name, u.last_name, u.avatar_url,
                        c.content, c.created_at
                 FROM comments c
                 JOIN users u ON u.id = c.user_id
                 WHERE c.post_id = $1
                 ORDER BY c.created_at ASC
                 LIMIT 20`, postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(
			&c.ID, &c.AuthorID, &c.AuthorFirstName, &c.AuthorLastName,
			&c.AuthorAvatarURL, &c.Content, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		c.RenderedContent = h.renderContent(r.Context(), c.Content)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// resolveHandle looks up a user id by email local-part (the @handle). Returns
// "" when no user matches.
func (h *Handler) resolveHandle(ctx context.Context, handle string) string {
	var id string
	h.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE lower(split_part(email,'@',1)) = $1`,
		strings.ToLower(handle),
	).Scan(&id)
	return id
}

// processMentions parses @handle tokens in content, resolves each to a user, and
// notifies every resolved user (except the author) with a "mention"
// notification. Failures are non-fatal.
func (h *Handler) processMentions(ctx context.Context, content, authorID string) {
	matches := handleMentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, m := range matches {
		handle := strings.ToLower(m[1])
		if seen[handle] {
			continue
		}
		seen[handle] = true

		mentionedID := h.resolveHandle(ctx, handle)
		if mentionedID == "" || mentionedID == authorID {
			continue
		}

		var firstName, lastName string
		h.db.QueryRowContext(ctx,
			`SELECT first_name, last_name FROM users WHERE id = $1`, authorID,
		).Scan(&firstName, &lastName)
		name := strings.TrimSpace(firstName + " " + lastName)
		if name == "" {
			name = "Someone"
		}
		h.notif.CreateNotification(ctx, mentionedID, authorID, "mention", name+" mentioned you in a post")
	}
}

// renderContent escapes content and linkifies @handle mentions to /profile/{id}
// (when the handle resolves to a user) and bare http(s) URLs to anchors. The
// returned value is safe HTML.
func (h *Handler) renderContent(ctx context.Context, content string) template.HTML {
	if markdown, ok := decodeRichMarkdown(content); ok {
		var rendered bytes.Buffer
		if err := goldmark.Convert([]byte(markdown), &rendered); err == nil {
			return template.HTML(rendered.String())
		}
	}

	// Escape first so user content can never inject markup; we then splice in
	// our own (trusted) anchor tags.
	escaped := template.HTMLEscapeString(content)

	// Linkify @handles. We resolve against the original (unescaped) handle text;
	// handle chars [a-z0-9._-] are unaffected by HTML escaping.
	escaped = handleMentionRegex.ReplaceAllStringFunc(escaped, func(tok string) string {
		handle := strings.ToLower(strings.TrimPrefix(tok, "@"))
		id := h.resolveHandle(ctx, handle)
		if id == "" {
			return `<span class="feed-mention">` + tok + `</span>`
		}
		return `<a class="feed-mention" href="/profile/` + template.HTMLEscapeString(id) + `">` + tok + `</a>`
	})

	// Linkify URLs (already HTML-escaped, so quotes etc. are safe in href).
	escaped = urlRegex.ReplaceAllStringFunc(escaped, func(u string) string {
		trimmed := strings.TrimRight(u, ".,;:!?)]}'\"")
		tail := u[len(trimmed):]
		return `<a class="feed-link" href="` + trimmed + `" target="_blank" rel="noopener noreferrer">` + trimmed + `</a>` + tail
	})

	return template.HTML(escaped)
}

func decodeRichMarkdown(content string) (string, bool) {
	if strings.HasPrefix(content, richMarkdownPrefix) {
		return strings.TrimPrefix(content, richMarkdownPrefix), true
	}
	return "", false
}

func encodeRichMarkdown(content string, format string) string {
	if format == "rich_markdown" {
		return richMarkdownPrefix + content
	}
	return content
}

func contentForProcessing(content string) string {
	if markdown, ok := decodeRichMarkdown(content); ok {
		// Lightweight normalization so mention/URL helpers can inspect source text.
		replacer := strings.NewReplacer("**", "", "*", "", "`", "", "[", "", "]", "", "(", " ", ")", "")
		return replacer.Replace(markdown)
	}
	return content
}

func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Fallback to regular form if no multipart
		if err2 := r.ParseForm(); err2 != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
		return
	}

	// Optional "Schedule for later": a future timestamp hides the post from
	// the feed until then. Past/blank/invalid values publish immediately (NULL).
	var scheduledAt interface{}
	if raw := strings.TrimSpace(r.FormValue("scheduled_at")); raw != "" {
		if t, perr := time.ParseInLocation("2006-01-02T15:04", raw, time.Local); perr == nil && t.After(time.Now()) {
			scheduledAt = t
		}
	}

	var postID string
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO posts (user_id, content, scheduled_at) VALUES ($1, $2, $3) RETURNING id`,
		userID, content, scheduledAt,
	).Scan(&postID)
	if err != nil {
		slog.Error("failed to create post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Extract and store hashtags
	if matches := hashtagRegex.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			tag := strings.ToLower(match[1])
			var hashtagID string
			err := h.db.QueryRowContext(r.Context(),
				`INSERT INTO hashtags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = $1 RETURNING id`,
				tag,
			).Scan(&hashtagID)
			if err == nil {
				h.db.ExecContext(r.Context(),
					`INSERT INTO post_hashtags (post_id, hashtag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
					postID, hashtagID,
				)
			}
		}
	}

	// Process @handle mentions and notify mentioned users (non-fatal).
	h.processMentions(r.Context(), content, userID)

	// If the post contains a URL, fetch and cache its Open Graph preview in the
	// background. Non-fatal: a failure simply means no preview card.
	if firstURL := extractFirstURL(content); firstURL != "" {
		go func(u string) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err := fetchAndCachePreview(ctx, h.db, u); err != nil {
				slog.Debug("link preview fetch failed", "url", u, "error", err)
			}
		}(firstURL)
	}

	// Handle file attachment (images and videos only for social posts)
	file, header, fileErr := r.FormFile("attachment")
	if fileErr == nil {
		defer file.Close()
		validated, valErr := attachment.ValidateFile(file, header, attachment.ContextPost)
		if valErr == nil {
			h.db.ExecContext(r.Context(),
				`INSERT INTO post_attachments (post_id, file_data, original_name, content_type, file_size, file_category)
                                 VALUES ($1, $2, $3, $4, $5, $6)`,
				postID, validated.Data, validated.OriginalName, validated.ContentType, validated.FileSize, string(validated.Category),
			)
		} else {
			slog.Error("attachment validation failed", "error", valErr)
		}
	}

	// Award badges (e.g. First Post). Non-fatal: never block post creation.
	badge.CheckAndAward(context.Background(), h.db, userID)

	go h.broker.Broadcast("")

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) serveAttachment(w http.ResponseWriter, r *http.Request) {
	attachID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.NotFound(w, r)
		return
	}

	var data []byte
	var contentType, originalName, fileCategory string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT pa.file_data, pa.content_type, pa.original_name, COALESCE(pa.file_category, 'image')
                 FROM post_attachments pa
                 JOIN posts p ON p.id = pa.post_id
                 WHERE pa.id = $1
                   AND (p.user_id = $2
                     OR p.user_id IN (
                       SELECT CASE
                         WHEN c.requester_id = $2 THEN c.addressee_id
                         ELSE c.requester_id
                       END
                       FROM connections c
                       WHERE (c.requester_id = $2 OR c.addressee_id = $2)
                         AND c.status = 'accepted'
                     ))`,
		attachID, userID,
	).Scan(&data, &contentType, &originalName, &fileCategory)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	attachment.ServeHeaders(w, contentType, originalName, attachment.Category(fileCategory))
	w.Write(data)
}

func (h *Handler) toggleLike(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	r.ParseForm()
	reactionType := r.FormValue("reaction")
	if reactionType == "" {
		reactionType = "like"
	}
	validReactions := map[string]bool{"like": true, "celebrate": true, "insightful": true, "curious": true}
	if !validReactions[reactionType] {
		reactionType = "like"
	}

	var existingReaction string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT reaction_type FROM post_likes WHERE post_id = $1 AND user_id = $2`,
		postID, userID,
	).Scan(&existingReaction)

	if err == nil {
		// Already reacted — if same type, remove; if different, update
		if existingReaction == reactionType {
			_, err = h.db.ExecContext(r.Context(),
				`DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`,
				postID, userID,
			)
		} else {
			_, err = h.db.ExecContext(r.Context(),
				`UPDATE post_likes SET reaction_type = $3 WHERE post_id = $1 AND user_id = $2`,
				postID, userID, reactionType,
			)
		}
	} else {
		_, err = h.db.ExecContext(r.Context(),
			`INSERT INTO post_likes (post_id, user_id, reaction_type) VALUES ($1, $2, $3)`,
			postID, userID, reactionType,
		)

		if err == nil {
			h.notifyPostAuthor(postID, userID, "reacted to your post")
		}
	}
	if err != nil {
		slog.Error("failed to toggle reaction", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	go h.broker.Broadcast(postID)

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO comments (post_id, user_id, content) VALUES ($1, $2, $3)`,
		postID, userID, content,
	)
	if err != nil {
		slog.Error("failed to add comment", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	} else {
		h.notifyPostAuthor(postID, userID, "commented on your post")
		// Process @handle mentions in the comment (non-fatal).
		h.processMentions(r.Context(), content, userID)
	}

	go h.broker.Broadcast(postID)

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM posts WHERE id = $1 AND user_id = $2`,
		postID, userID,
	)
	if err != nil {
		slog.Error("failed to delete post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	go h.broker.Broadcast(postID)

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) sharePost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	postID := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	commentary := strings.TrimSpace(r.FormValue("commentary"))

	var originalContent string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT content FROM posts WHERE id = $1`,
		postID,
	).Scan(&originalContent)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sharedContent := "Shared a post:\n\n" + originalContent
	if commentary != "" {
		sharedContent = commentary + "\n\n---\n" + originalContent
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("failed to begin share transaction", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO shared_posts (user_id, original_post_id, commentary) VALUES ($1, $2, $3)`,
		userID, postID, commentary,
	)
	if err != nil {
		slog.Error("failed to share post", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO posts (user_id, content, scheduled_at) VALUES ($1, $2, $3)`,
		userID, sharedContent, nil,
	)
	if err != nil {
		slog.Error("failed to create feed post for share", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit share transaction", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.notifyPostAuthor(postID, userID, "shared your post")
	go h.broker.Broadcast("")

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

type HashtagPage struct {
	middleware.BaseData
	Tag   string
	Posts []Post
}

func (h *Handler) showHashtagFeed(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tag := r.PathValue("tag")

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT p.id, p.user_id, u.first_name, u.last_name, u.avatar_url, u.headline,
                        p.content, p.created_at,
                        (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
                        (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
                        EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1),
                        COALESCE((SELECT reaction_type FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = $1), '')
                 FROM posts p
                 JOIN users u ON u.id = p.user_id
                 JOIN post_hashtags ph ON ph.post_id = p.id
                 JOIN hashtags h ON h.id = ph.hashtag_id AND LOWER(h.name) = LOWER($2)
                 ORDER BY p.created_at DESC
                 LIMIT 50`, userID, tag,
	)
	if err != nil {
		slog.Error("failed to load hashtag feed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(
			&p.ID, &p.AuthorID, &p.AuthorFirstName, &p.AuthorLastName,
			&p.AuthorAvatarURL, &p.AuthorHeadline,
			&p.Content, &p.CreatedAt,
			&p.LikeCount, &p.CommentCount, &p.LikedByUser, &p.UserReaction,
		); err != nil {
			continue
		}
		p.IsAuthor = p.AuthorID == userID
		posts = append(posts, p)
	}

	data := HashtagPage{BaseData: middleware.NewBaseData(r), Tag: tag, Posts: posts}
	h.pages["hashtag.html"].ExecuteTemplate(w, "base", data)
}

type TrendingTag struct {
	Name      string
	PostCount int
	Score     int
}

type TrendingPage struct {
	middleware.BaseData
	Tags []TrendingTag
}

// showTrending ranks hashtags by recent activity over a rolling 7-day window.
//
// Ranking approach: for each hashtag we window on the linked post's created_at
// (last 7 days via the post_hashtags join table) and compute two figures —
//   - PostCount: distinct recent posts carrying the tag, and
//   - Score: an engagement-weighted signal = posts + likes + comments on those
//     recent posts, so tags driving conversation outrank merely frequent ones.
//
// Tags are ordered by Score (then PostCount) descending and the top 15 are
// rendered. With no recent activity the result set is empty and the template
// shows its empty state.
func (h *Handler) showTrending(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT h.name,
                        COUNT(DISTINCT p.id) AS post_count,
                        (COUNT(DISTINCT p.id)
                         + COUNT(DISTINCT pl.id)
                         + COUNT(DISTINCT c.id)) AS score
                 FROM hashtags h
                 JOIN post_hashtags ph ON ph.hashtag_id = h.id
                 JOIN posts p ON p.id = ph.post_id
                     AND p.created_at > NOW() - INTERVAL '7 days'
                     AND (p.scheduled_at IS NULL OR p.scheduled_at <= NOW())
                 LEFT JOIN post_likes pl ON pl.post_id = p.id
                 LEFT JOIN comments c ON c.post_id = p.id
                 GROUP BY h.name
                 ORDER BY score DESC, post_count DESC, h.name ASC
                 LIMIT 15`,
	)
	if err != nil {
		slog.Error("failed to load trending", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tags []TrendingTag
	for rows.Next() {
		var t TrendingTag
		if err := rows.Scan(&t.Name, &t.PostCount, &t.Score); err != nil {
			slog.Error("failed to scan trending row", "error", err)
			continue
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed to read trending rows", "error", err)
	}

	data := TrendingPage{BaseData: middleware.NewBaseData(r), Tags: tags}
	if err := h.pages["trending.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render trending", "error", err)
	}
}

type ScheduledPost struct {
	ID              string
	Content         string
	RenderedContent template.HTML
	CreatedAt       time.Time
	ScheduledAt     time.Time
}

type Draft struct {
	ID              string
	Content         string
	RenderedContent template.HTML
	CreatedAt       time.Time
}

type DraftsPage struct {
	middleware.BaseData
	Drafts         []Draft
	ScheduledPosts []ScheduledPost
}

func (h *Handler) redirectScheduledToDrafts(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
}

func (h *Handler) showDrafts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	draftRows, err := h.db.QueryContext(r.Context(),
		`SELECT id, content, created_at FROM post_drafts WHERE user_id = $1 ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load drafts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer draftRows.Close()

	var drafts []Draft
	for draftRows.Next() {
		var d Draft
		if err := draftRows.Scan(&d.ID, &d.Content, &d.CreatedAt); err != nil {
			continue
		}
		d.RenderedContent = h.renderContent(r.Context(), d.Content)
		drafts = append(drafts, d)
	}

	scheduledRows, err := h.db.QueryContext(r.Context(),
		`SELECT id, content, created_at, scheduled_at
                 FROM posts
                 WHERE user_id = $1 AND scheduled_at IS NOT NULL AND scheduled_at > now()
                 ORDER BY scheduled_at ASC`,
		userID,
	)
	if err != nil {
		slog.Error("failed to load scheduled posts for drafts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer scheduledRows.Close()

	var scheduledPosts []ScheduledPost
	for scheduledRows.Next() {
		var p ScheduledPost
		if err := scheduledRows.Scan(&p.ID, &p.Content, &p.CreatedAt, &p.ScheduledAt); err != nil {
			continue
		}
		p.RenderedContent = h.renderContent(r.Context(), p.Content)
		scheduledPosts = append(scheduledPosts, p)
	}

	data := DraftsPage{BaseData: middleware.NewBaseData(r), Drafts: drafts, ScheduledPosts: scheduledPosts}
	h.pages["drafts.html"].ExecuteTemplate(w, "base", data)
}

func (h *Handler) saveDraft(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	r.ParseForm()
	rawContent := strings.TrimSpace(r.FormValue("content"))
	content := encodeRichMarkdown(rawContent, strings.TrimSpace(r.FormValue("content_format")))
	if content == "" {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
		return
	}
	h.db.ExecContext(r.Context(),
		`INSERT INTO post_drafts (user_id, content) VALUES ($1, $2)`,
		userID, content,
	)
	http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
}

func (h *Handler) scheduleDraftPost(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	content = encodeRichMarkdown(content, strings.TrimSpace(r.FormValue("content_format")))
	if content == "" {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
		return
	}

	rawScheduledAt := strings.TrimSpace(r.FormValue("scheduled_at"))
	scheduledAt, err := time.ParseInLocation("2006-01-02T15:04", rawScheduledAt, time.Local)
	if err != nil || !scheduledAt.After(time.Now()) {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO posts (user_id, content, scheduled_at) VALUES ($1, $2, $3)`,
		userID, content, scheduledAt,
	)
	if err != nil {
		slog.Error("failed to schedule post from drafts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
}

func (h *Handler) publishDraft(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	draftID := r.PathValue("id")

	var content string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT content FROM post_drafts WHERE id = $1 AND user_id = $2`,
		draftID, userID,
	).Scan(&content)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h.db.ExecContext(r.Context(),
		`INSERT INTO posts (user_id, content) VALUES ($1, $2)`,
		userID, content,
	)
	h.db.ExecContext(r.Context(),
		`DELETE FROM post_drafts WHERE id = $1`, draftID,
	)
	go h.broker.Broadcast("")
	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) deleteDraft(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	draftID := r.PathValue("id")
	h.db.ExecContext(r.Context(),
		`DELETE FROM post_drafts WHERE id = $1 AND user_id = $2`,
		draftID, userID,
	)
	http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
}
