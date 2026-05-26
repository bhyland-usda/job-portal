package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	articlepkg "github.com/bhyland-usda/job-portal/internal/article"
	bookmarkpkg "github.com/bhyland-usda/job-portal/internal/bookmark"
	connectionpkg "github.com/bhyland-usda/job-portal/internal/connection"
	foiapkg "github.com/bhyland-usda/job-portal/internal/foia"
	grouppkg "github.com/bhyland-usda/job-portal/internal/group"
	mentorshippkg "github.com/bhyland-usda/job-portal/internal/mentorship"
	moderationpkg "github.com/bhyland-usda/job-portal/internal/moderation"
	pollpkg "github.com/bhyland-usda/job-portal/internal/poll"
	postingpkg "github.com/bhyland-usda/job-portal/internal/posting"
	searchpkg "github.com/bhyland-usda/job-portal/internal/search"
	userpkg "github.com/bhyland-usda/job-portal/internal/user"
	workspacepkg "github.com/bhyland-usda/job-portal/internal/workspace"
)

var workflowFixtures = newFixtureState()

type fixtureState struct {
	mu sync.Mutex

	sequence int

	notifications map[string][]fixtureNotification
	connections   map[string]*fixtureConnection
	conversations map[string]*fixtureConversation
	pins          map[string]map[string]bool
	groups        map[string]*fixtureGroup
	mentorships   map[string]*fixtureMentorship
	workspaces    map[string]*fixtureWorkspace
	bookmarks     map[string][]bookmarkpkg.Bookmark
	polls         map[string]*fixturePoll
	reports       map[string]*moderationpkg.Report
	activities    []fixtureActivity

	nextConversationID int
	nextMessageID      int
	nextNotificationID int
	nextMentorshipID   int
	nextGroupPostID    int
	nextWorkspaceNote  int
	nextBookmarkID     int
}

type fixtureNotification struct {
	ID             string
	ClickURL       string
	SenderAvatar   string
	SenderInitials string
	Message        string
	TimeAgo        string
	Read           bool
	CreatedAt      time.Time
}

type fixtureConnection struct {
	RequesterID string
	AddresseeID string
	Status      string
	UpdatedAt   time.Time
}

type fixtureConversation struct {
	ID           string
	Name         string
	Participants map[string]bool
	Messages     []*fixtureMessage
	Unread       map[string]int
	UpdatedAt    time.Time
}

type fixtureMessage struct {
	ID        string
	SenderID  string
	Content   string
	CreatedAt time.Time
	ReadAt    time.Time
}

type fixtureGroup struct {
	Group   grouppkg.Group
	Members map[string]bool
	Posts   []grouppkg.GroupPost
}

type fixtureMentorship struct {
	mentorshippkg.Mentorship
}

type fixtureWorkspace struct {
	Workspace workspacepkg.Workspace
	Members   []workspacepkg.Member
	Notes     []workspacepkg.Note
}

type fixturePoll struct {
	ID        string
	AuthorID  string
	Question  string
	CreatedAt time.Time
	Options   []fixturePollOption
	Votes     map[string]string
}

type fixturePollOption struct {
	ID    string
	Label string
}

type fixtureActivity struct {
	Type      string
	AuthorID  string
	Author    string
	Content   string
	CreatedAt time.Time
}

func newFixtureState() *fixtureState {
	s := &fixtureState{
		notifications: make(map[string][]fixtureNotification),
		connections:   make(map[string]*fixtureConnection),
		conversations: make(map[string]*fixtureConversation),
		pins:          make(map[string]map[string]bool),
		groups:        make(map[string]*fixtureGroup),
		mentorships:   make(map[string]*fixtureMentorship),
		workspaces:    make(map[string]*fixtureWorkspace),
		bookmarks:     make(map[string][]bookmarkpkg.Bookmark),
		polls:         make(map[string]*fixturePoll),
		reports:       make(map[string]*moderationpkg.Report),
	}

	s.notifications["employee-1"] = []fixtureNotification{
		{
			ID:             "notif-1",
			ClickURL:       "/feed?highlight=abcde-1234",
			SenderInitials: "TJ",
			Message:        "Taylor Jordan mentioned you in a post.",
			TimeAgo:        "2 minutes ago",
			Read:           false,
			CreatedAt:      fixtureNow.Add(-2 * time.Minute),
		},
		{
			ID:             "notif-2",
			ClickURL:       "/connections",
			SenderInitials: "CB",
			Message:        "Casey Brooks sent you a connection request.",
			TimeAgo:        "10 minutes ago",
			Read:           false,
			CreatedAt:      fixtureNow.Add(-10 * time.Minute),
		},
		{
			ID:             "notif-3",
			ClickURL:       "/announcements",
			SenderInitials: "SP",
			Message:        "A new workforce planning announcement was posted.",
			TimeAgo:        "1 hour ago",
			Read:           true,
			CreatedAt:      fixtureNow.Add(-1 * time.Hour),
		},
	}
	s.notifications["manager-1"] = []fixtureNotification{
		{
			ID:             "notif-4",
			ClickURL:       "/postings/posting-1",
			SenderInitials: "RC",
			Message:        "Riley Carter bookmarked your detail posting.",
			TimeAgo:        "30 minutes ago",
			Read:           true,
			CreatedAt:      fixtureNow.Add(-30 * time.Minute),
		},
	}
	s.notifications["admin-1"] = []fixtureNotification{
		{
			ID:             "notif-5",
			ClickURL:       "/admin/moderation",
			SenderInitials: "ML",
			Message:        "A new moderation report is ready for review.",
			TimeAgo:        "15 minutes ago",
			Read:           false,
			CreatedAt:      fixtureNow.Add(-15 * time.Minute),
		},
	}
	s.nextNotificationID = 6

	s.connections[s.connectionKey("employee-1", "mentor-1")] = &fixtureConnection{
		RequesterID: "employee-1",
		AddresseeID: "mentor-1",
		Status:      "accepted",
		UpdatedAt:   fixtureNow.Add(-72 * time.Hour),
	}
	s.connections[s.connectionKey("employee-1", "coworker-1")] = &fixtureConnection{
		RequesterID: "coworker-1",
		AddresseeID: "employee-1",
		Status:      "accepted",
		UpdatedAt:   fixtureNow.Add(-48 * time.Hour),
	}
	s.connections[s.connectionKey("employee-1", "diana-1")] = &fixtureConnection{
		RequesterID: "diana-1",
		AddresseeID: "employee-1",
		Status:      "accepted",
		UpdatedAt:   fixtureNow.Add(-36 * time.Hour),
	}
	s.connections[s.connectionKey("employee-1", "coworker-2")] = &fixtureConnection{
		RequesterID: "coworker-2",
		AddresseeID: "employee-1",
		Status:      "pending",
		UpdatedAt:   fixtureNow.Add(-20 * time.Minute),
	}

	s.conversations["conv-1"] = &fixtureConversation{
		ID: "conv-1",
		Participants: map[string]bool{
			"employee-1": true,
			"mentor-1":   true,
		},
		Messages: []*fixtureMessage{
			{
				ID:        "msg-1",
				SenderID:  "mentor-1",
				Content:   "Sounds good. I can pull the org chart snapshot this afternoon.",
				CreatedAt: fixtureNow.Add(-90 * time.Minute),
				ReadAt:    fixtureNow.Add(-80 * time.Minute),
			},
		},
		Unread: map[string]int{
			"employee-1": 0,
			"mentor-1":   0,
		},
		UpdatedAt: fixtureNow.Add(-90 * time.Minute),
	}
	s.nextConversationID = 2
	s.nextMessageID = 2
	s.pins["employee-1"] = map[string]bool{"mentor-1": true}

	s.groups["group-1"] = &fixtureGroup{
		Group: grouppkg.Group{
			ID:          "group-1",
			Name:        "USDA Analytics Guild",
			Description: "Cross-agency workspace for analytics standards and peer coaching.",
			MemberCount: 3,
		},
		Members: map[string]bool{
			"employee-1": true,
			"mentor-1":   true,
			"coworker-1": true,
		},
		Posts: []grouppkg.GroupPost{{
			ID:         "group-post-1",
			UserID:     "mentor-1",
			AuthorName: "Morgan Lee",
			Content:    "Shared a draft dashboard review checklist for manager feedback.",
			CreatedAt:  fixtureNow.Add(-6 * time.Hour),
		}},
	}
	s.nextGroupPostID = 2

	s.mentorships["mentorship-1"] = &fixtureMentorship{
		Mentorship: mentorshippkg.Mentorship{
			ID:         "mentorship-1",
			MentorID:   "mentor-1",
			MentorName: "Morgan Lee",
			MenteeID:   "employee-1",
			MenteeName: "Riley Carter",
			Status:     "active",
			CreatedAt:  fixtureNow.AddDate(0, -2, 0),
		},
	}
	s.nextMentorshipID = 2

	s.workspaces["workspace-1"] = &fixtureWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:          "workspace-1",
			Name:        "Field Analytics Sprint",
			Description: "Working group for workforce dashboard content and rollout notes.",
			MemberCount: 3,
		},
		Members: []workspacepkg.Member{
			{UserID: "employee-1", Name: "Riley Carter", JoinedAt: fixtureNow.AddDate(0, -1, 0)},
			{UserID: "mentor-1", Name: "Morgan Lee", JoinedAt: fixtureNow.AddDate(0, -1, -2)},
			{UserID: "coworker-1", Name: "Alex Nguyen", JoinedAt: fixtureNow.AddDate(0, -1, -4)},
		},
		Notes: []workspacepkg.Note{{
			ID:         "note-1",
			AuthorID:   "mentor-1",
			AuthorName: "Morgan Lee",
			Body:       "Uploaded the latest rollout notes for the department leads review.",
			CreatedAt:  fixtureNow.Add(-90 * time.Minute),
		}},
	}
	s.nextWorkspaceNote = 2

	s.bookmarks["employee-1"] = []bookmarkpkg.Bookmark{{
		ID:         "bookmark-1",
		UserID:     "employee-1",
		TargetType: "article",
		TargetID:   "article-1",
		CreatedAt:  fixtureNow.Add(-48 * time.Hour),
		Title:      "Building agency-wide analytics partnerships",
	}}
	s.nextBookmarkID = 2

	s.polls["poll-1"] = &fixturePoll{
		ID:        "poll-1",
		AuthorID:  "manager-1",
		Question:  "Which launch cadence works best for the new workforce dashboard?",
		CreatedAt: fixtureNow.AddDate(0, 0, -3),
		Options: []fixturePollOption{
			{ID: "opt-1", Label: "Weekly"},
			{ID: "opt-2", Label: "Biweekly"},
			{ID: "opt-3", Label: "Monthly"},
		},
		Votes: map[string]string{
			"mentor-1":   "opt-1",
			"coworker-1": "opt-1",
			"coworker-2": "opt-2",
			"manager-1":  "opt-2",
			"admin-1":    "opt-3",
		},
	}

	s.reports["report-1"] = &moderationpkg.Report{
		ID:           "report-1",
		ReporterID:   "mentor-1",
		ReporterName: "Morgan Lee",
		ContentType:  "post",
		ContentID:    "abcde-1234",
		Reason:       "Contains outdated staffing numbers.",
		Status:       "pending",
		CreatedAt:    fixtureNow.Add(-4 * time.Hour),
	}

	s.activities = []fixtureActivity{
		{
			Type:      "post",
			AuthorID:  "employee-1",
			Author:    "Riley Carter",
			Content:   "Shared workforce pilot notes with agency reviewers.",
			CreatedAt: fixtureNow.AddDate(0, 0, -2),
		},
		{
			Type:      "comment",
			AuthorID:  "coworker-1",
			Author:    "Alex Nguyen",
			Content:   "Thanks for posting this update.",
			CreatedAt: fixtureNow.AddDate(0, 0, -2).Add(30 * time.Minute),
		},
		{
			Type:      "message",
			AuthorID:  "mentor-1",
			Author:    "Morgan Lee",
			Content:   "Sounds good. I can pull the org chart snapshot this afternoon.",
			CreatedAt: fixtureNow.Add(-90 * time.Minute),
		},
	}

	return s
}

func (s *fixtureState) nextTimeLocked() time.Time {
	s.sequence++
	return fixtureNow.Add(time.Duration(s.sequence) * time.Minute)
}

func (s *fixtureState) connectionKey(userA, userB string) string {
	users := []string{userA, userB}
	sort.Strings(users)
	return users[0] + "|" + users[1]
}

func (s *fixtureState) connectionStatusLocked(currentUserID, otherUserID string) string {
	if currentUserID == otherUserID {
		return "self"
	}
	conn, ok := s.connections[s.connectionKey(currentUserID, otherUserID)]
	if !ok || conn.Status == "rejected" {
		return "none"
	}
	if conn.Status == "accepted" {
		return "connected"
	}
	if conn.RequesterID == currentUserID {
		return "pending_sent"
	}
	return "pending_received"
}

func (s *fixtureState) connectionStatus(currentUserID, otherUserID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connectionStatusLocked(currentUserID, otherUserID)
}

func (s *fixtureState) bookmarksPage(current fixtureUser, tab string) bookmarkpkg.BookmarkPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	bookmarks := append([]bookmarkpkg.Bookmark(nil), s.bookmarks[current.ID]...)
	filtered := make([]bookmarkpkg.Bookmark, 0, len(bookmarks))
	for _, item := range bookmarks {
		item.Title = s.bookmarkTitleLocked(item.TargetType, item.TargetID)
		if tab == "all" || item.TargetType == tab {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	return bookmarkpkg.BookmarkPage{
		BaseData:  current.baseData(),
		Tab:       tab,
		Bookmarks: filtered,
	}
}

func (s *fixtureState) bookmarkTitleLocked(targetType, targetID string) string {
	switch targetType {
	case "posting":
		return "Temporary detail opportunity"
	case "article":
		return "Building agency-wide analytics partnerships"
	default:
		return strings.Title(targetType) + " " + targetID
	}
}

func (s *fixtureState) isBookmarked(currentUserID, targetType, targetID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, bookmark := range s.bookmarks[currentUserID] {
		if bookmark.TargetType == targetType && bookmark.TargetID == targetID {
			return true
		}
	}
	return false
}

func (s *fixtureState) handleBookmarkAdd(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	targetType := strings.TrimSpace(r.FormValue("target_type"))
	targetID := strings.TrimSpace(r.FormValue("target_id"))
	if targetType == "" || targetID == "" {
		http.Error(w, "target_type and target_id are required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, bookmark := range s.bookmarks[current.ID] {
		if bookmark.TargetType == targetType && bookmark.TargetID == targetID {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	createdAt := s.nextTimeLocked()
	bookmark := bookmarkpkg.Bookmark{
		ID:         fmt.Sprintf("bookmark-%d", s.nextBookmarkID),
		UserID:     current.ID,
		TargetType: targetType,
		TargetID:   targetID,
		CreatedAt:  createdAt,
		Title:      s.bookmarkTitleLocked(targetType, targetID),
	}
	s.nextBookmarkID++
	s.bookmarks[current.ID] = append([]bookmarkpkg.Bookmark{bookmark}, s.bookmarks[current.ID]...)

	if current.ID != "manager-1" {
		s.addNotificationLocked("manager-1", "/postings/"+targetID, current, current.FirstName+" "+current.LastName+" bookmarked your "+targetType+".")
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) handleBookmarkDelete(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	bookmarkID := r.PathValue("id")

	s.mu.Lock()
	bookmarks := s.bookmarks[current.ID]
	filtered := bookmarks[:0]
	for _, bookmark := range bookmarks {
		if bookmark.ID != bookmarkID {
			filtered = append(filtered, bookmark)
		}
	}
	s.bookmarks[current.ID] = filtered
	s.mu.Unlock()

	redirectBack(w, r, "/bookmarks")
}

func (s *fixtureState) notificationsCount(currentUserID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, notification := range s.notifications[currentUserID] {
		if !notification.Read {
			count++
		}
	}
	return count
}

func (s *fixtureState) notificationsList(currentUserID string) []fixtureNotification {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := append([]fixtureNotification(nil), s.notifications[currentUserID]...)
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list
}

func (s *fixtureState) handleNotificationsCount(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	writeJSON(w, map[string]int{"count": s.notificationsCount(current.ID)})
}

func (s *fixtureState) handleNotificationsRecent(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	list := s.notificationsList(current.ID)
	payload := make([]map[string]any, 0, len(list))
	for _, notification := range list {
		payload = append(payload, map[string]any{
			"id":              notification.ID,
			"click_url":       notification.ClickURL,
			"sender_avatar":   notification.SenderAvatar,
			"sender_initials": notification.SenderInitials,
			"message":         notification.Message,
			"time_ago":        notification.TimeAgo,
			"read":            notification.Read,
		})
	}
	writeJSON(w, payload)
}

func (s *fixtureState) handleNotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	s.mu.Lock()
	for i := range s.notifications[current.ID] {
		s.notifications[current.ID][i].Read = true
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) handleNotificationRead(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	notificationID := r.PathValue("id")
	s.mu.Lock()
	for i := range s.notifications[current.ID] {
		if s.notifications[current.ID][i].ID == notificationID {
			s.notifications[current.ID][i].Read = true
			break
		}
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) handleNotificationDelete(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	notificationID := r.PathValue("id")
	s.mu.Lock()
	notifications := s.notifications[current.ID]
	filtered := notifications[:0]
	for _, notification := range notifications {
		if notification.ID != notificationID {
			filtered = append(filtered, notification)
		}
	}
	s.notifications[current.ID] = filtered
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) connectionsPage(current fixtureUser) connectionpkg.ConnectionsPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pending []connectionpkg.ConnectionUser
	var accepted []connectionpkg.ConnectionUser
	for _, conn := range s.connections {
		switch {
		case conn.Status == "pending" && conn.AddresseeID == current.ID:
			user := lookupUser(conn.RequesterID)
			pending = append(pending, connectionpkg.ConnectionUser{
				ID:        user.ID,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Headline:  user.Headline,
				AvatarURL: user.AvatarURL,
			})
		case conn.Status == "accepted" && (conn.RequesterID == current.ID || conn.AddresseeID == current.ID):
			otherID := conn.RequesterID
			if otherID == current.ID {
				otherID = conn.AddresseeID
			}
			user := lookupUser(otherID)
			accepted = append(accepted, connectionpkg.ConnectionUser{
				ID:        user.ID,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Headline:  user.Headline,
				AvatarURL: user.AvatarURL,
			})
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].FirstName+pending[i].LastName < pending[j].FirstName+pending[j].LastName
	})
	sort.Slice(accepted, func(i, j int) bool {
		return accepted[i].FirstName+accepted[i].LastName < accepted[j].FirstName+accepted[j].LastName
	})

	return connectionpkg.ConnectionsPage{
		BaseData: current.baseData(),
		Pending:  pending,
		Accepted: accepted,
	}
}

func (s *fixtureState) handleConnectionRequest(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	targetID := r.PathValue("id")
	if targetID == "" || targetID == current.ID {
		http.Error(w, "invalid connection target", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.connectionKey(current.ID, targetID)
	conn, ok := s.connections[key]
	if !ok {
		conn = &fixtureConnection{
			RequesterID: current.ID,
			AddresseeID: targetID,
			Status:      "pending",
			UpdatedAt:   s.nextTimeLocked(),
		}
		s.connections[key] = conn
		s.addNotificationLocked(targetID, "/connections", current, current.FirstName+" "+current.LastName+" sent you a connection request.")
	}

	redirectBack(w, r, "/connections")
}

func (s *fixtureState) handleConnectionAccept(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	requesterID := r.PathValue("id")

	s.mu.Lock()
	key := s.connectionKey(current.ID, requesterID)
	if conn, ok := s.connections[key]; ok && conn.Status == "pending" && conn.AddresseeID == current.ID {
		conn.Status = "accepted"
		conn.UpdatedAt = s.nextTimeLocked()
		s.addNotificationLocked(requesterID, "/connections", current, current.FirstName+" "+current.LastName+" accepted your connection request.")
	}
	s.mu.Unlock()

	redirectBack(w, r, "/connections")
}

func (s *fixtureState) handleConnectionReject(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	requesterID := r.PathValue("id")

	s.mu.Lock()
	key := s.connectionKey(current.ID, requesterID)
	if conn, ok := s.connections[key]; ok && conn.Status == "pending" && conn.AddresseeID == current.ID {
		delete(s.connections, key)
	}
	s.mu.Unlock()

	redirectBack(w, r, "/connections")
}

func (s *fixtureState) searchPage(current fixtureUser, query string) searchpkg.SearchPage {
	if strings.TrimSpace(query) == "" {
		query = "analytics"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	needle := strings.ToLower(strings.TrimSpace(query))
	results := make([]searchpkg.SearchResult, 0)
	for _, user := range directoryUsers {
		if user.ID == current.ID {
			continue
		}
		haystack := strings.ToLower(user.FirstName + " " + user.LastName + " " + user.Headline + " " + user.Location)
		if needle != "" && !strings.Contains(haystack, needle) {
			continue
		}
		results = append(results, searchpkg.SearchResult{
			ID:               user.ID,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			Headline:         user.Headline,
			AvatarURL:        user.AvatarURL,
			Location:         user.Location,
			ConnectionStatus: s.connectionStatusLocked(current.ID, user.ID),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		left := results[i].FirstName + " " + results[i].LastName
		right := results[j].FirstName + " " + results[j].LastName
		return left < right
	})

	return searchpkg.SearchPage{
		BaseData: current.baseData(),
		Query:    query,
		Searched: true,
		Results:  results,
	}
}

func (s *fixtureState) profileConnectionStatus(current fixtureUser, subjectID string, isOwn bool) string {
	if isOwn {
		return "self"
	}
	return s.connectionStatus(current.ID, subjectID)
}

func (s *fixtureState) groupPage(current fixtureUser, groupID string) grouppkg.GroupPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		group = s.groups["group-1"]
	}

	pageGroup := group.Group
	pageGroup.MemberCount = len(group.Members)
	posts := append([]grouppkg.GroupPost(nil), group.Posts...)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	return grouppkg.GroupPage{
		BaseData: current.baseData(),
		Group:    pageGroup,
		IsMember: group.Members[current.ID],
		Posts:    posts,
	}
}

func (s *fixtureState) handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	groupID := r.PathValue("id")

	s.mu.Lock()
	if group, ok := s.groups[groupID]; ok {
		group.Members[current.ID] = true
		group.Group.MemberCount = len(group.Members)
	}
	s.mu.Unlock()

	redirectBack(w, r, "/groups/"+groupID)
}

func (s *fixtureState) handleGroupLeave(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	groupID := r.PathValue("id")

	s.mu.Lock()
	if group, ok := s.groups[groupID]; ok {
		delete(group.Members, current.ID)
		group.Group.MemberCount = len(group.Members)
	}
	s.mu.Unlock()

	redirectBack(w, r, "/groups/"+groupID)
}

func (s *fixtureState) handleGroupPost(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	groupID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))

	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groups[groupID]
	if !ok || !group.Members[current.ID] || content == "" {
		redirectBack(w, r, "/groups/"+groupID)
		return
	}

	post := grouppkg.GroupPost{
		ID:         fmt.Sprintf("group-post-%d", s.nextGroupPostID),
		UserID:     current.ID,
		AuthorName: current.FirstName + " " + current.LastName,
		Content:    content,
		CreatedAt:  s.nextTimeLocked(),
	}
	s.nextGroupPostID++
	group.Posts = append([]grouppkg.GroupPost{post}, group.Posts...)

	redirectBack(w, r, "/groups/"+groupID)
}

func (s *fixtureState) mentorshipPage(current fixtureUser, tab string) mentorshippkg.MentorPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var mentorships []mentorshippkg.Mentorship
	for _, mentorship := range s.mentorships {
		if mentorship.MentorID == current.ID || mentorship.MenteeID == current.ID {
			if mentorship.Status == "pending" || mentorship.Status == "active" {
				mentorships = append(mentorships, mentorship.Mentorship)
			}
		}
	}
	sort.Slice(mentorships, func(i, j int) bool {
		return mentorships[i].CreatedAt.After(mentorships[j].CreatedAt)
	})

	available := make([]mentorshippkg.Mentor, 0)
	if tab == "find" {
		for _, candidateID := range []string{"mentor-1", "mentor-2", "manager-1"} {
			candidate := lookupUser(candidateID)
			if candidate.ID == current.ID {
				continue
			}
			skip := false
			for _, mentorship := range s.mentorships {
				if mentorship.MenteeID == current.ID && mentorship.MentorID == candidate.ID && (mentorship.Status == "pending" || mentorship.Status == "active") {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			available = append(available, mentorshippkg.Mentor{
				ID:           candidate.ID,
				FirstName:    candidate.FirstName,
				LastName:     candidate.LastName,
				Headline:     candidate.Headline,
				SkillCount:   10 + len(candidate.FirstName),
				SharedSkills: len(candidate.LastName) % 4,
			})
		}
	}

	return mentorshippkg.MentorPage{
		BaseData:         current.baseData(),
		Tab:              tab,
		Mentorships:      mentorships,
		AvailableMentors: available,
	}
}

func (s *fixtureState) handleMentorRequest(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	mentor := lookupUser(r.PathValue("userID"))
	if mentor.ID == current.ID {
		http.Error(w, "cannot request yourself", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mentorship := range s.mentorships {
		if mentorship.MentorID == mentor.ID && mentorship.MenteeID == current.ID && (mentorship.Status == "pending" || mentorship.Status == "active") {
			redirectBack(w, r, "/mentorship?tab=active")
			return
		}
	}

	id := fmt.Sprintf("mentorship-%d", s.nextMentorshipID)
	s.nextMentorshipID++
	createdAt := s.nextTimeLocked()
	s.mentorships[id] = &fixtureMentorship{
		Mentorship: mentorshippkg.Mentorship{
			ID:         id,
			MentorID:   mentor.ID,
			MentorName: mentor.FirstName + " " + mentor.LastName,
			MenteeID:   current.ID,
			MenteeName: current.FirstName + " " + current.LastName,
			Status:     "pending",
			CreatedAt:  createdAt,
		},
	}
	s.addNotificationLocked(mentor.ID, "/mentorship", current, current.FirstName+" "+current.LastName+" requested you as a mentor.")

	redirectBack(w, r, "/mentorship?tab=active")
}

func (s *fixtureState) handleMentorshipAccept(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	mentorshipID := r.PathValue("id")

	s.mu.Lock()
	if mentorship, ok := s.mentorships[mentorshipID]; ok && mentorship.MentorID == current.ID && mentorship.Status == "pending" {
		mentorship.Status = "active"
		s.addNotificationLocked(mentorship.MenteeID, "/mentorship", current, current.FirstName+" "+current.LastName+" accepted your mentorship request.")
	}
	s.mu.Unlock()

	redirectBack(w, r, "/mentorship")
}

func (s *fixtureState) handleMentorshipDecline(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	mentorshipID := r.PathValue("id")

	s.mu.Lock()
	if mentorship, ok := s.mentorships[mentorshipID]; ok && mentorship.MentorID == current.ID && mentorship.Status == "pending" {
		delete(s.mentorships, mentorshipID)
	}
	s.mu.Unlock()

	redirectBack(w, r, "/mentorship")
}

func (s *fixtureState) handleMentorshipComplete(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	mentorshipID := r.PathValue("id")

	s.mu.Lock()
	if mentorship, ok := s.mentorships[mentorshipID]; ok && mentorship.Status == "active" && (mentorship.MentorID == current.ID || mentorship.MenteeID == current.ID) {
		mentorship.Status = "completed"
	}
	s.mu.Unlock()

	redirectBack(w, r, "/mentorship")
}

func (s *fixtureState) workspacePage(current fixtureUser) workspacepkg.ViewPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspace := s.workspaces["workspace-1"]
	pageWorkspace := workspace.Workspace
	pageWorkspace.MemberCount = len(workspace.Members)
	members := append([]workspacepkg.Member(nil), workspace.Members...)
	notes := append([]workspacepkg.Note(nil), workspace.Notes...)
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})

	return workspacepkg.ViewPage{
		BaseData:  current.baseData(),
		Workspace: pageWorkspace,
		Members:   members,
		Notes:     notes,
	}
}

func (s *fixtureState) handleWorkspaceNote(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	workspaceID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))

	s.mu.Lock()
	defer s.mu.Unlock()

	workspace, ok := s.workspaces[workspaceID]
	if !ok || body == "" {
		redirectBack(w, r, "/workspaces/"+workspaceID)
		return
	}

	note := workspacepkg.Note{
		ID:         fmt.Sprintf("note-%d", s.nextWorkspaceNote),
		AuthorID:   current.ID,
		AuthorName: current.FirstName + " " + current.LastName,
		Body:       body,
		CreatedAt:  s.nextTimeLocked(),
	}
	s.nextWorkspaceNote++
	workspace.Notes = append([]workspacepkg.Note{note}, workspace.Notes...)

	redirectBack(w, r, "/workspaces/"+workspaceID)
}

func (s *fixtureState) handleWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	workspaceID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	addedUser, ok := directoryUsers[userID]
	if !ok {
		redirectBack(w, r, "/workspaces/"+workspaceID)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	workspace, ok := s.workspaces[workspaceID]
	if !ok {
		redirectBack(w, r, "/workspaces/"+workspaceID)
		return
	}
	for _, member := range workspace.Members {
		if member.UserID == addedUser.ID {
			redirectBack(w, r, "/workspaces/"+workspaceID)
			return
		}
	}
	workspace.Members = append(workspace.Members, workspacepkg.Member{
		UserID:   addedUser.ID,
		Name:     addedUser.FirstName + " " + addedUser.LastName,
		JoinedAt: s.nextTimeLocked(),
	})
	workspace.Workspace.MemberCount = len(workspace.Members)
	s.addNotificationLocked(addedUser.ID, "/workspaces/"+workspaceID, current, current.FirstName+" "+current.LastName+" added you to the workspace.")

	redirectBack(w, r, "/workspaces/"+workspaceID)
}

func (s *fixtureState) postingPage(current fixtureUser) postingpkg.PostingPage {
	author := lookupUser("manager-1")
	if current.Role == "manager" || current.Role == "admin" {
		author = current
	}
	return postingpkg.PostingPage{
		BaseData: current.baseData(),
		Posting: postingpkg.Posting{
			ID:            "posting-1",
			AuthorID:      author.ID,
			AuthorName:    author.FirstName + " " + author.LastName,
			Title:         "Temporary detail opportunity",
			Description:   "Support the USDA workforce dashboard refresh and synthesize field office feedback for leadership.",
			Type:          "detail",
			Location:      "Washington, DC",
			Department:    "NRCS",
			Status:        "active",
			Skills:        []string{"Program Management", "Analytics", "Stakeholder Engagement"},
			MatchCount:    14,
			CreatedAt:     fixtureNow.AddDate(0, 0, -3),
			Outcome:       "Selected candidate begins the 120-day detail in July.",
			HasOutcome:    true,
			CompletedAt:   fixtureNow.AddDate(0, 0, -1),
			OutcomeStatus: "completed",
		},
		Bookmarked: s.isBookmarked(current.ID, "posting", "posting-1"),
	}
}

func (s *fixtureState) articlePage(current fixtureUser) articlepkg.ViewPage {
	author := lookupUser("manager-1")
	return articlepkg.ViewPage{
		BaseData: current.baseData(),
		Article: articlepkg.Article{
			ID:         "article-1",
			AuthorID:   author.ID,
			AuthorName: author.FirstName + " " + author.LastName,
			Title:      "Building agency-wide analytics partnerships",
			Content:    "This representative article uses the real article template to exercise long-form content rendering.",
			CreatedAt:  fixtureNow.AddDate(0, 0, -4),
		},
		IsAuthor:   current.ID == author.ID,
		Bookmarked: s.isBookmarked(current.ID, "article", "article-1"),
	}
}

func (s *fixtureState) pollPage(current fixtureUser) pollpkg.PollPage {
	s.mu.Lock()
	defer s.mu.Unlock()

	page := pollpkg.PollPage{BaseData: current.baseData()}
	for _, poll := range s.polls {
		optionCounts := make(map[string]int, len(poll.Options))
		for _, vote := range poll.Votes {
			optionCounts[vote]++
		}
		totalVotes := len(poll.Votes)
		options := make([]pollpkg.PollOption, 0, len(poll.Options))
		for index, option := range poll.Options {
			count := optionCounts[option.ID]
			percent := 0
			if totalVotes > 0 {
				percent = count * 100 / totalVotes
			}
			options = append(options, pollpkg.PollOption{
				ID:        option.ID,
				Label:     option.Label,
				SortOrder: index,
				VoteCount: count,
				Percent:   percent,
			})
		}
		page.Polls = append(page.Polls, pollpkg.Poll{
			ID:                poll.ID,
			AuthorID:          poll.AuthorID,
			AuthorName:        lookupUser(poll.AuthorID).FirstName + " " + lookupUser(poll.AuthorID).LastName,
			Question:          poll.Question,
			CreatedAt:         poll.CreatedAt,
			Options:           options,
			TotalVotes:        totalVotes,
			UserVotedOptionID: poll.Votes[current.ID],
		})
	}
	sort.Slice(page.Polls, func(i, j int) bool {
		return page.Polls[i].CreatedAt.After(page.Polls[j].CreatedAt)
	})
	return page
}

func (s *fixtureState) handlePollVote(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	pollID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	optionID := strings.TrimSpace(r.FormValue("option"))

	s.mu.Lock()
	if poll, ok := s.polls[pollID]; ok {
		for _, option := range poll.Options {
			if option.ID == optionID {
				poll.Votes[current.ID] = optionID
				break
			}
		}
	}
	s.mu.Unlock()

	redirectBack(w, r, "/polls")
}

func (s *fixtureState) moderationPage(current fixtureUser) moderationpkg.QueuePage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var reports []moderationpkg.Report
	for _, report := range s.reports {
		if report.Status == "pending" {
			reports = append(reports, *report)
		}
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	return moderationpkg.QueuePage{
		BaseData: current.baseData(),
		Reports:  reports,
	}
}

func (s *fixtureState) handleModerationResolve(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	action := strings.TrimSpace(r.FormValue("action"))
	if action == "" {
		action = "reviewed"
	}

	s.mu.Lock()
	if report, ok := s.reports[reportID]; ok {
		report.Status = action
	}
	s.mu.Unlock()

	redirectBack(w, r, "/admin/moderation")
}

func (s *fixtureState) foiaPage(current fixtureUser, r *http.Request, searched bool) foiapkg.SearchPage {
	userName := strings.TrimSpace(r.URL.Query().Get("user"))
	dateFrom := strings.TrimSpace(r.URL.Query().Get("from"))
	dateTo := strings.TrimSpace(r.URL.Query().Get("to"))
	contentType := strings.TrimSpace(r.URL.Query().Get("type"))

	page := foiapkg.SearchPage{
		BaseData: current.baseData(),
		UserName: userName,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Type:     contentType,
		Searched: searched,
	}
	if searched {
		page.Results = s.foiaResults(userName, dateFrom, dateTo, contentType)
	}
	return page
}

func (s *fixtureState) foiaResults(userName, dateFrom, dateTo, contentType string) []foiapkg.SearchResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	var fromTime time.Time
	if dateFrom != "" {
		fromTime, _ = time.Parse("2006-01-02", dateFrom)
	}
	var toTime time.Time
	if dateTo != "" {
		toTime, _ = time.Parse("2006-01-02", dateTo)
		toTime = toTime.Add(24*time.Hour - time.Nanosecond)
	}

	needle := strings.ToLower(strings.TrimSpace(userName))
	results := make([]foiapkg.SearchResult, 0)
	for _, activity := range s.activities {
		if contentType != "" && activity.Type != contentType {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(activity.Author), needle) {
			continue
		}
		if !fromTime.IsZero() && activity.CreatedAt.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && activity.CreatedAt.After(toTime) {
			continue
		}
		results = append(results, foiapkg.SearchResult{
			Type:      activity.Type,
			AuthorID:  activity.AuthorID,
			Author:    activity.Author,
			Content:   activity.Content,
			CreatedAt: activity.CreatedAt,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results
}

func (s *fixtureState) handleFOIAExport(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "admin")
	results := s.foiaResults(
		strings.TrimSpace(r.URL.Query().Get("user")),
		strings.TrimSpace(r.URL.Query().Get("from")),
		strings.TrimSpace(r.URL.Query().Get("to")),
		strings.TrimSpace(r.URL.Query().Get("type")),
	)

	writeJSON(w, map[string]any{
		"export_id":    "foia-export-1",
		"status":       "ready",
		"requested_by": current.ID,
		"count":        len(results),
		"results":      results,
	})
}

func (s *fixtureState) handleMessageRecent(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")

	s.mu.Lock()
	defer s.mu.Unlock()

	pinned := make([]map[string]any, 0)
	for userID := range s.pins[current.ID] {
		user := lookupUser(userID)
		pinned = append(pinned, map[string]any{
			"id":   user.ID,
			"name": user.FirstName + " " + user.LastName,
		})
	}
	sort.Slice(pinned, func(i, j int) bool {
		return pinned[i]["name"].(string) < pinned[j]["name"].(string)
	})

	type recentConversation struct {
		ID          string
		Name        string
		LastMessage string
		Unread      int
		OtherUserID string
		Pinned      bool
		IsGroup     bool
		UpdatedAt   time.Time
	}
	recent := make([]recentConversation, 0)
	for _, conv := range s.conversations {
		if !conv.Participants[current.ID] {
			continue
		}
		lastMessage := ""
		if len(conv.Messages) > 0 {
			lastMessage = conv.Messages[len(conv.Messages)-1].Content
		}
		otherUserID := ""
		if len(conv.Participants) == 2 {
			for participant := range conv.Participants {
				if participant != current.ID {
					otherUserID = participant
					break
				}
			}
		}
		recent = append(recent, recentConversation{
			ID:          conv.ID,
			Name:        s.conversationNameLocked(conv, current.ID),
			LastMessage: lastMessage,
			Unread:      conv.Unread[current.ID],
			OtherUserID: otherUserID,
			Pinned:      s.pins[current.ID][otherUserID],
			IsGroup:     len(conv.Participants) > 2 || conv.Name != "",
			UpdatedAt:   conv.UpdatedAt,
		})
	}
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].UpdatedAt.After(recent[j].UpdatedAt)
	})

	payload := make([]map[string]any, 0, len(recent))
	for _, conv := range recent {
		payload = append(payload, map[string]any{
			"id":            conv.ID,
			"name":          conv.Name,
			"last_message":  conv.LastMessage,
			"unread":        conv.Unread,
			"other_user_id": conv.OtherUserID,
			"pinned":        conv.Pinned,
			"is_group":      conv.IsGroup,
		})
	}

	writeJSON(w, map[string]any{
		"pinned": pinned,
		"recent": payload,
	})
}

func (s *fixtureState) handleMessageSearchUsers(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	results := make([]map[string]string, 0)
	for _, user := range directoryUsers {
		if user.ID == current.ID {
			continue
		}
		name := user.FirstName + " " + user.LastName
		if query != "" && !strings.Contains(strings.ToLower(name+" "+user.Headline+" "+user.Location), query) {
			continue
		}
		results = append(results, map[string]string{
			"id":   user.ID,
			"name": name,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i]["name"] < results[j]["name"]
	})
	writeJSON(w, results)
}

func (s *fixtureState) handleMessageStart(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	userIDs := r.Form["user_ids"]
	selected := make([]string, 0, len(userIDs))
	seen := map[string]bool{current.ID: true}
	for _, userID := range userIDs {
		if _, ok := directoryUsers[userID]; ok && !seen[userID] {
			seen[userID] = true
			selected = append(selected, userID)
		}
	}
	if len(selected) == 0 {
		http.Error(w, "at least one user is required", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	s.mu.Lock()
	defer s.mu.Unlock()

	participants := make(map[string]bool, len(selected)+1)
	participants[current.ID] = true
	for _, userID := range selected {
		participants[userID] = true
	}

	if len(participants) == 2 && name == "" {
		if existing := s.findDirectConversationLocked(current.ID, selected[0]); existing != nil {
			writeJSON(w, map[string]string{"id": existing.ID})
			return
		}
	}

	id := fmt.Sprintf("conv-%d", s.nextConversationID)
	s.nextConversationID++
	s.conversations[id] = &fixtureConversation{
		ID:           id,
		Name:         name,
		Participants: participants,
		Messages:     nil,
		Unread:       map[string]int{},
		UpdatedAt:    s.nextTimeLocked(),
	}

	writeJSON(w, map[string]string{"id": id})
}

func (s *fixtureState) handleMessageNew(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	otherID := r.PathValue("id")
	if _, ok := directoryUsers[otherID]; !ok || otherID == current.ID {
		http.Error(w, "invalid participant", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing := s.findDirectConversationLocked(current.ID, otherID); existing != nil {
		writeJSON(w, map[string]string{"id": existing.ID})
		return
	}

	id := fmt.Sprintf("conv-%d", s.nextConversationID)
	s.nextConversationID++
	s.conversations[id] = &fixtureConversation{
		ID: id,
		Participants: map[string]bool{
			current.ID: true,
			otherID:    true,
		},
		Unread:    map[string]int{},
		UpdatedAt: s.nextTimeLocked(),
	}
	writeJSON(w, map[string]string{"id": id})
}

func (s *fixtureState) handleMessageChatGet(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	conversationID := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok || !conv.Participants[current.ID] {
		http.NotFound(w, r)
		return
	}

	conv.Unread[current.ID] = 0
	readAt := s.nextTimeLocked()
	for _, message := range conv.Messages {
		if message.SenderID != current.ID {
			message.ReadAt = readAt
		}
	}

	w.Header().Set("X-Conversation-Name", s.conversationNameLocked(conv, current.ID))
	payload := make([]map[string]any, 0, len(conv.Messages))
	for _, message := range conv.Messages {
		sender := lookupUser(message.SenderID)
		item := map[string]any{
			"id":         message.ID,
			"name":       sender.FirstName + " " + sender.LastName,
			"content":    message.Content,
			"is_own":     message.SenderID == current.ID,
			"created_at": message.CreatedAt.Format("3:04 PM"),
		}
		if message.SenderID == current.ID && !message.ReadAt.IsZero() {
			item["read_at"] = "Read"
		}
		payload = append(payload, item)
	}

	writeJSON(w, payload)
}

func (s *fixtureState) handleMessageChatPost(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	conversationID := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok || !conv.Participants[current.ID] {
		http.NotFound(w, r)
		return
	}

	content := ""
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err == nil {
			content = strings.TrimSpace(r.FormValue("content"))
			if content == "" && r.MultipartForm != nil && len(r.MultipartForm.File["attachment"]) > 0 {
				content = "Shared attachment: " + r.MultipartForm.File["attachment"][0].Filename
			}
		}
	} else if err := r.ParseForm(); err == nil {
		content = strings.TrimSpace(r.FormValue("content"))
	}
	if content == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	createdAt := s.nextTimeLocked()
	message := &fixtureMessage{
		ID:        fmt.Sprintf("msg-%d", s.nextMessageID),
		SenderID:  current.ID,
		Content:   content,
		CreatedAt: createdAt,
	}
	s.nextMessageID++
	conv.Messages = append(conv.Messages, message)
	conv.UpdatedAt = createdAt
	for participant := range conv.Participants {
		if participant != current.ID {
			conv.Unread[participant]++
		}
	}
	s.activities = append(s.activities, fixtureActivity{
		Type:      "message",
		AuthorID:  current.ID,
		Author:    current.FirstName + " " + current.LastName,
		Content:   content,
		CreatedAt: createdAt,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) handleMessageAddMember(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	conversationID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if _, ok := directoryUsers[userID]; !ok || userID == current.ID {
		http.Error(w, "invalid participant", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok || !conv.Participants[current.ID] {
		http.NotFound(w, r)
		return
	}
	conv.Participants[userID] = true
	conv.UpdatedAt = s.nextTimeLocked()

	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) handleMessagePin(w http.ResponseWriter, r *http.Request) {
	current := selectedFixtureUser(r, "employee")
	userID := r.PathValue("id")
	if _, ok := directoryUsers[userID]; !ok || userID == current.ID {
		http.Error(w, "invalid participant", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pins[current.ID] == nil {
		s.pins[current.ID] = make(map[string]bool)
	}
	if s.pins[current.ID][userID] {
		delete(s.pins[current.ID], userID)
	} else {
		s.pins[current.ID][userID] = true
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *fixtureState) findDirectConversationLocked(userA, userB string) *fixtureConversation {
	for _, conv := range s.conversations {
		if len(conv.Participants) != 2 {
			continue
		}
		if conv.Participants[userA] && conv.Participants[userB] {
			return conv
		}
	}
	return nil
}

func (s *fixtureState) conversationNameLocked(conv *fixtureConversation, viewerID string) string {
	if conv.Name != "" {
		return conv.Name
	}
	names := make([]string, 0, len(conv.Participants))
	for participant := range conv.Participants {
		if participant == viewerID {
			continue
		}
		user := lookupUser(participant)
		names = append(names, user.FirstName+" "+user.LastName)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func (s *fixtureState) addNotificationLocked(userID, clickURL string, sender fixtureUser, message string) {
	createdAt := s.nextTimeLocked()
	notification := fixtureNotification{
		ID:             fmt.Sprintf("notif-%d", s.nextNotificationID),
		ClickURL:       clickURL,
		SenderInitials: string(sender.FirstName[0]) + string(sender.LastName[0]),
		Message:        message,
		TimeAgo:        "just now",
		Read:           false,
		CreatedAt:      createdAt,
	}
	s.nextNotificationID++
	s.notifications[userID] = append([]fixtureNotification{notification}, s.notifications[userID]...)
}

func (s *fixtureState) exportStateJSON() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := map[string]any{
		"notifications": s.notifications,
		"connections":   s.connections,
		"activities":    s.activities,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func (s *fixtureState) profileView(current fixtureUser, profileID string) userpkg.ProfileView {
	subject := current
	if profileID != "" && profileID != current.ID && profileID != "me" {
		subject = lookupUser(profileID)
	}
	isOwn := subject.ID == current.ID || profileID == "me"
	connectionStatus := s.profileConnectionStatus(current, subject.ID, isOwn)
	birthday := time.Date(1990, time.July, 12, 0, 0, 0, 0, time.UTC)
	hireDate := time.Date(2019, time.March, 4, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, time.December, 1, 0, 0, 0, 0, time.UTC)
	endYear := 2018
	data := &userpkg.ProfileData{
		BaseData: current.baseData(),
		User: userpkg.User{
			ID:         subject.ID,
			FirstName:  subject.FirstName,
			LastName:   subject.LastName,
			Headline:   subject.Headline,
			About:      subject.About,
			Location:   subject.Location,
			Birthday:   &birthday,
			HireDate:   &hireDate,
			WorkStatus: "telework",
		},
		Experiences: []userpkg.Experience{{
			ID:          "exp-1",
			Title:       "Program Manager",
			Company:     "USDA",
			Location:    subject.Location,
			StartDate:   time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndDate:     &endDate,
			Description: "Led employee engagement pilots across multiple service centers.",
		}},
		Educations: []userpkg.Education{{
			ID:           "edu-1",
			School:       "Iowa State University",
			Degree:       "M.S.",
			FieldOfStudy: "Public Administration",
			StartYear:    2016,
			EndYear:      &endYear,
		}},
		IsOwnProfile:     isOwn,
		ConnectionStatus: connectionStatus,
		Completeness: &userpkg.ProfileCompleteness{
			Score:   5,
			Total:   7,
			Percent: 71,
			Items: []userpkg.CompletenessItem{
				{Label: "Add a headline", Complete: true},
				{Label: "Add a location", Complete: true},
				{Label: "Write an about section", Complete: true},
				{Label: "Upload a photo", Complete: false},
				{Label: "Add experience", Complete: true},
				{Label: "Add education", Complete: true},
				{Label: "Add skills", Complete: false},
			},
		},
	}
	view := userpkg.ProfileView{
		ProfileData: data,
		IsFollowing: !isOwn,
		SkillViews: []userpkg.SkillView{{
			Skill:             userpkg.Skill{ID: "skill-1", Name: "Program Management", EndorsementCount: 4, EndorsedByUser: !isOwn},
			VerificationCount: 2,
		}, {
			Skill:             userpkg.Skill{ID: "skill-2", Name: "Analytics", EndorsementCount: 3},
			VerificationCount: 1,
		}},
	}
	if isOwn {
		view.PinnedPosts = []userpkg.PinnedPost{{
			ID:              "pinned-1",
			AuthorID:        current.ID,
			AuthorFirstName: current.FirstName,
			AuthorLastName:  current.LastName,
			Content:         "Pinned post for a representative profile fixture.",
			CreatedAt:       fixtureNow.Add(-48 * time.Hour),
			PinnedAt:        fixtureNow.Add(-24 * time.Hour),
		}}
		view.Celebrations = []userpkg.Celebration{
			{UserID: "coworker-1", FirstName: "Alex", LastName: "Nguyen", Kind: "birthday"},
			{UserID: "mentor-1", FirstName: "Morgan", LastName: "Lee", Kind: "anniversary", Years: 6},
		}
	}
	return view
}
