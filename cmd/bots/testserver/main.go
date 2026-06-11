package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	accomplishmentpkg "github.com/bhyland-usda/job-portal/internal/accomplishment"
	adminpkg "github.com/bhyland-usda/job-portal/internal/admin"
	analyticspkg "github.com/bhyland-usda/job-portal/internal/analytics"
	announcementpkg "github.com/bhyland-usda/job-portal/internal/announcement"
	articlepkg "github.com/bhyland-usda/job-portal/internal/article"
	badgepkg "github.com/bhyland-usda/job-portal/internal/badge"
	bookmarkpkg "github.com/bhyland-usda/job-portal/internal/bookmark"
	certificationpkg "github.com/bhyland-usda/job-portal/internal/certification"
	connectionpkg "github.com/bhyland-usda/job-portal/internal/connection"
	departmentpkg "github.com/bhyland-usda/job-portal/internal/department"
	digestpkg "github.com/bhyland-usda/job-portal/internal/digest"
	feedbackpkg "github.com/bhyland-usda/job-portal/internal/feedback"
	foiapkg "github.com/bhyland-usda/job-portal/internal/foia"
	grouppkg "github.com/bhyland-usda/job-portal/internal/group"
	insightspkg "github.com/bhyland-usda/job-portal/internal/insights"
	kudospkg "github.com/bhyland-usda/job-portal/internal/kudos"
	mentorshippkg "github.com/bhyland-usda/job-portal/internal/mentorship"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	moderationpkg "github.com/bhyland-usda/job-portal/internal/moderation"
	newspkg "github.com/bhyland-usda/job-portal/internal/news"
	onboardingpkg "github.com/bhyland-usda/job-portal/internal/onboarding"
	postingpkg "github.com/bhyland-usda/job-portal/internal/opportunity"
	orgchartpkg "github.com/bhyland-usda/job-portal/internal/orgchart"
	pollpkg "github.com/bhyland-usda/job-portal/internal/poll"
	resumepkg "github.com/bhyland-usda/job-portal/internal/resume"
	searchpkg "github.com/bhyland-usda/job-portal/internal/search"
	spotlightpkg "github.com/bhyland-usda/job-portal/internal/spotlight"
	userpkg "github.com/bhyland-usda/job-portal/internal/user"
	workspacepkg "github.com/bhyland-usda/job-portal/internal/workspace"
)

var fixtureNow = time.Date(2026, time.May, 23, 15, 30, 0, 0, time.UTC)

var fixtureUsers = map[string]fixtureUser{
	"employee": {
		ID:        "employee-1",
		FirstName: "Riley",
		LastName:  "Carter",
		Headline:  "Soil Conservationist",
		Location:  "Des Moines, IA",
		About:     "Supports conservation planning and cross-agency field training.",
		Role:      "employee",
	},
	"manager": {
		ID:        "manager-1",
		FirstName: "Taylor",
		LastName:  "Jordan",
		Headline:  "Branch Chief",
		Location:  "Washington, DC",
		About:     "Leads workforce planning for career mobility and internal talent programs.",
		Role:      "manager",
		AvatarURL: "/avatar/manager-1",
	},
	"admin": {
		ID:        "admin-1",
		FirstName: "Sam",
		LastName:  "Patel",
		Headline:  "Platform Administrator",
		Location:  "Beltsville, MD",
		About:     "Maintains employee engagement, compliance, and reporting tools across the portal.",
		Role:      "admin",
		AvatarURL: "/avatar/admin-1",
	},
}

var directoryUsers = map[string]fixtureUser{
	"employee-1": fixtureUsers["employee"],
	"manager-1":  fixtureUsers["manager"],
	"admin-1":    fixtureUsers["admin"],
	"mentor-1": {
		ID:        "mentor-1",
		FirstName: "Morgan",
		LastName:  "Lee",
		Headline:  "Program Analyst",
		Location:  "Fort Collins, CO",
		About:     "Focuses on internal mobility, mentoring, and program delivery.",
		Role:      "employee",
		AvatarURL: "/avatar/mentor-1",
	},
	"coworker-1": {
		ID:        "coworker-1",
		FirstName: "Alex",
		LastName:  "Nguyen",
		Headline:  "GIS Specialist",
		Location:  "Lincoln, NE",
		About:     "Builds maps, dashboards, and field enablement content.",
		Role:      "employee",
		AvatarURL: "/avatar/coworker-1",
	},
	"coworker-2": {
		ID:        "coworker-2",
		FirstName: "Casey",
		LastName:  "Brooks",
		Headline:  "Communications Lead",
		Location:  "Kansas City, MO",
		About:     "Shares employee stories, announcements, and workforce updates.",
		Role:      "employee",
	},
	"mentor-2": {
		ID:        "mentor-2",
		FirstName: "Jordan",
		LastName:  "Kim",
		Headline:  "Data Scientist",
		Location:  "Raleigh, NC",
		About:     "Advises teams on analytics, hiring insights, and reusable data products.",
		Role:      "employee",
	},
	"diana-1": {
		ID:        "diana-1",
		FirstName: "Diana",
		LastName:  "Prince",
		Headline:  "Program Specialist",
		Location:  "Arlington, VA",
		About:     "Coordinates leadership development and rotational detail programs.",
		Role:      "employee",
		AvatarURL: "/avatar/diana-1",
	},
}

type fixtureUser struct {
	ID        string
	FirstName string
	LastName  string
	Headline  string
	Location  string
	About     string
	Role      string
	AvatarURL string
}

type fixtureLink struct {
	Label string
	URL   string
}

type shellPage struct {
	middleware.BaseData
	Links []fixtureLink
}

type feedPage struct {
	middleware.BaseData
	Tab      string
	Posts    []feedPost
	Postings []feedPosting
}

type feedDraftPage struct {
	middleware.BaseData
	Drafts         []feedDraft
	ScheduledPosts []scheduledFeedPost
}

type feedDraft struct {
	ID              string
	Content         string
	RenderedContent template.HTML
	CreatedAt       time.Time
}

type scheduledFeedPost struct {
	ID              string
	Content         string
	RenderedContent template.HTML
	CreatedAt       time.Time
	ScheduledAt     time.Time
}

type feedPost struct {
	ID              string
	AuthorID        string
	AuthorAvatarURL string
	AuthorFirstName string
	AuthorLastName  string
	AuthorHeadline  string
	IsAuthor        bool
	RenderedContent template.HTML
	Content         string
	LikeCount       int
	CommentCount    int
	UserReaction    string
	Bookmarked      bool
	Preview         *feedPreview
	Attachments     []feedAttachment
	Comments        []feedComment
}

type feedComment struct {
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	RenderedContent template.HTML
	Content         string
}

type feedPreview struct {
	URL         string
	ImageURL    string
	Title       string
	Description string
}

type feedAttachment struct {
	ID           string
	Category     string
	ContentType  string
	OriginalName string
}

type feedPosting struct {
	ID          string
	Type        string
	AuthorName  string
	Title       string
	Department  string
	Location    string
	Description string
	CreatedAt   fixtureTime
}

type fixtureTime struct {
	label string
}

type resumeExperience struct {
	Title       string
	Company     string
	Location    string
	Description string
	StartDate   time.Time
	EndDate     sql.NullTime
}

type resumeEducation struct {
	School       string
	Degree       string
	FieldOfStudy string
	StartYear    int
	EndYear      sql.NullInt64
}

type aupPage struct {
	middleware.BaseData
	Accepted bool
}

type notificationPreferencesPage struct {
	middleware.BaseData
	Saved bool
	Prefs []notificationPreference
}

type notificationPreference struct {
	Type    string
	Label   string
	Enabled bool
}

type sessionsPage struct {
	middleware.BaseData
	Revoked  bool
	Sessions []sessionFixture
}

type sessionFixture struct {
	UserAgent string
	CreatedAt string
	MaskedID  string
	Token     string
	IsCurrent bool
}

type templateFixture struct {
	Pattern     string
	Page        string
	ExecuteName string
	DefaultRole string
	Build       func(fixtureUser, *http.Request) any
}

func (t fixtureTime) Format(string) string {
	return t.label
}

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "4173"
	}

	pages := map[string]*template.Template{
		"login.html":                    parseTemplate("auth/login.html"),
		"sessions.html":                 parseTemplate("auth/sessions.html"),
		"profile.html":                  parseTemplate("profile/view.html"),
		"profile_edit.html":             parseTemplate("profile/edit.html"),
		"resumes.html":                  parseTemplate("resume/index.html"),
		"resume_view.html":              template.Must(template.ParseFiles("templates/resume/view.html")),
		"accomplishments.html":          parseTemplate("accomplishment/index.html"),
		"accomplishment_form.html":      parseTemplate("accomplishment/form.html"),
		"certifications.html":           parseTemplate("certification/index.html"),
		"digest.html":                   parseTemplate("digest/index.html"),
		"feedback_request.html":         parseTemplate("feedback/request.html"),
		"search.html":                   parseTemplate("search/index.html"),
		"onboarding.html":               parseTemplate("onboarding/index.html"),
		"connections.html":              parseTemplate("connections/index.html"),
		"notification_preferences.html": parseTemplate("notification/preferences.html"),
		"bookmarks.html":                parseTemplate("bookmark/index.html"),
		"group_view.html":               parseTemplate("group/view.html"),
		"mentorship.html":               parseTemplate("mentorship/index.html"),
		"department_view.html":          parseTemplate("department/view.html"),
		"workspace_view.html":           parseTemplate("workspace/view.html"),
		"posting_view.html":             parseTemplate("opportunity/view.html"),
		"posting_matches.html":          parseTemplate("opportunity/matches.html"),
		"posting_search.html":           parseTemplate("opportunity/search.html"),
		"posting_create.html":           parseTemplate("opportunity/create.html"),
		"posting_applications.html":     parseTemplate("opportunity/applications.html"),
		"posting_outcome.html":          parseTemplate("opportunity/outcome.html"),
		"drafts.html":                   parseTemplate("feed/drafts.html"),
		"article_view.html":             parseTemplate("article/view.html"),
		"news_view.html":                parseTemplate("news/view.html"),
		"announcements.html":            parseTemplate("announcement/index.html"),
		"spotlight.html":                parseTemplate("spotlight/index.html"),
		"spotlight_create.html":         parseTemplate("spotlight/create.html"),
		"kudos.html":                    parseTemplate("kudos/index.html"),
		"polls.html":                    parseTemplate("poll/index.html"),
		"badges.html":                   parseTemplate("badge/index.html"),
		"admin_dashboard.html":          parseTemplate("admin/dashboard.html"),
		"admin_audit.html":              parseTemplate("admin/audit.html"),
		"admin_report.html":             template.Must(template.ParseFiles("templates/admin/report.html")),
		"admin_users.html":              parseTemplate("admin/users.html"),
		"moderation_queue.html":         parseTemplate("moderation/queue.html"),
		"foia.html":                     parseTemplate("admin/foia.html"),
		"workforce_dashboard.html":      parseTemplate("analytics/workforce_dashboard.html"),
		"leaderboard.html":              parseTemplate("analytics/leaderboard.html"),
		"heatmap.html":                  parseTemplate("insights/heatmap.html"),
		"network.html":                  parseTemplate("insights/network.html"),
		"orgchart.html":                 parseTemplate("orgchart/index.html"),
		"orgchart_assign.html":          parseTemplate("orgchart/assign.html"),
		"data_export.html":              parseTemplate("dataexport/index.html"),
		"aup.html":                      parseTemplate("aup/index.html"),
	}

	shellTmpl := template.Must(template.ParseFiles(
		"templates/layouts/base.html",
		"templates/layouts/navbar.html",
	))
	template.Must(shellTmpl.Parse(`
{{define "title"}}Shell Fixture{{end}}
{{define "content"}}
<section class="profile-card">
  <h1>Shared shell fixture</h1>
  <p>This page renders the real USDA shell templates for Playwright coverage.</p>
  <div style="display:grid;gap:8px;margin-top:16px;">
    {{range .Links}}
    <a href="{{.URL}}" class="title-link">{{.Label}}</a>
    {{end}}
  </div>
</section>
<section class="profile-card">
  <h2>Scrollable content</h2>
  <p>Used to verify the sticky header and overlay positioning.</p>
  <div style="height: 1400px;"></div>
</section>
{{end}}
`))
	feedTmpl := template.Must(template.ParseFiles(
		"templates/layouts/base.html",
		"templates/layouts/navbar.html",
		"templates/feed/feed.html",
	))
	landingTmpl := template.Must(template.ParseFiles("templates/landing.html"))

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/landing", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /avatar/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := directoryUsers[r.PathValue("id")]
		if !ok || user.AvatarURL == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96" role="img" aria-label="%s"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%%" stop-color="#1a6aaa"/><stop offset="100%%" stop-color="#005941"/></linearGradient></defs><circle cx="48" cy="48" r="48" fill="url(#g)"/><text x="50%%" y="54%%" text-anchor="middle" font-family="Source Sans 3, Source Sans Pro, Arial, sans-serif" font-size="34" font-weight="700" fill="#ffffff">%s</text></svg>`, user.FirstName+" "+user.LastName, string(user.FirstName[0])+string(user.LastName[0]))
	})
	mux.HandleFunc("POST /__reset", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures = newFixtureState()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /shell", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		data := shellPage{
			BaseData: current.baseData(),
			Links: []fixtureLink{
				{Label: "Employee profile fixture", URL: "/profile/me?role=employee"},
				{Label: "Manager posting fixture", URL: "/opportunities/posting-1?role=manager"},
				{Label: "Admin dashboard fixture", URL: "/admin/dashboard?role=admin"},
				{Label: "Search fixture", URL: "/search?role=employee&q=analytics"},
				{Label: "Workspace fixture", URL: "/workspaces/workspace-1?role=employee"},
				{Label: "Org chart fixture", URL: "/orgchart?role=admin"},
			},
		}
		if err := shellTmpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /landing", func(w http.ResponseWriter, r *http.Request) {
		if err := landingTmpl.ExecuteTemplate(w, "landing", nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			middleware.BaseData
			Error string
		}{Error: r.URL.Query().Get("error")}
		if err := pages["login.html"].ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
			return
		}
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		if email == "invalid.user@usda.gov" || password == "definitely-wrong-password" || email == "" || password == "" {
			http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
			return
		}
		current := selectFixtureUser(w, r, "employee")
		setRoleCookie(w, current.Role)
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: roleCookieName, Value: "", Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /feed", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		tab := r.URL.Query().Get("tab")
		if tab == "" {
			tab = "social"
		}
		if err := feedTmpl.ExecuteTemplate(w, "base", sampleFeedPage(current, tab)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /feed/drafts", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["drafts.html"].ExecuteTemplate(w, "base", sampleFeedDraftPage(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fixtures := []templateFixture{
		{Pattern: "/profile", Page: "profile.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleProfileView(current, current.ID)
		}},
		{Pattern: "/profile/me", Page: "profile.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleProfileView(current, current.ID)
		}},
		{Pattern: "/resume", Page: "resumes.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleResumeIndex(current)
		}},
		{Pattern: "/profile/{id}", Page: "profile.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleProfileView(current, r.PathValue("id"))
		}},
		{Pattern: "/resumes/generate", Page: "resume_view.html", ExecuteName: "resume_base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleResumeData(current)
		}},
		{Pattern: "/accomplishments", Page: "accomplishments.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			period := r.URL.Query().Get("period")
			if period == "" {
				period = "quarterly"
			}
			return sampleAccomplishments(current, period)
		}},
		{Pattern: "/certifications", Page: "certifications.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleCertifications(current)
		}},
		{Pattern: "/digest", Page: "digest.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleDigest(current)
		}},
		{Pattern: "/feedback-ask/{userID}", Page: "feedback_request.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleFeedbackRequest(current, r.PathValue("userID"))
		}},
		{Pattern: "/feedback/request", Page: "feedback_request.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleFeedbackRequest(current, "mentor-1")
		}},
		{Pattern: "/search", Page: "search.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleSearch(current, r.URL.Query().Get("q"))
		}},
		{Pattern: "/onboarding", Page: "onboarding.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleOnboarding(current)
		}},
		{Pattern: "/connections", Page: "connections.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleConnections(current)
		}},
		{Pattern: "/notifications/preferences", Page: "notification_preferences.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleNotificationPreferences(current, r.URL.Query().Get("saved") == "1")
		}},
		{Pattern: "/settings/sessions", Page: "sessions.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleSessions(current, r.URL.Query().Get("revoked") == "1")
		}},
		{Pattern: "/bookmarks", Page: "bookmarks.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			tab := r.URL.Query().Get("tab")
			if tab == "" {
				tab = "all"
			}
			return sampleBookmarks(current, tab)
		}},
		{Pattern: "/groups/{id}", Page: "group_view.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleGroup(current, r.PathValue("id"))
		}},
		{Pattern: "/mentorship", Page: "mentorship.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			tab := r.URL.Query().Get("tab")
			if tab == "" {
				tab = "active"
			}
			return sampleMentorship(current, tab)
		}},
		{Pattern: "/departments/{id}", Page: "department_view.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleDepartment(current)
		}},
		{Pattern: "/workspaces/{id}", Page: "workspace_view.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleWorkspace(current)
		}},
		{Pattern: "/opportunities/{id}", Page: "posting_view.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return samplePosting(current)
		}},
		{Pattern: "/articles/{id}", Page: "article_view.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return sampleArticle(current)
		}},
		{Pattern: "/news/{id}", Page: "news_view.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return sampleNews(current)
		}},
		{Pattern: "/announcements", Page: "announcements.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return sampleAnnouncements(current)
		}},
		{Pattern: "/spotlight", Page: "spotlight.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return sampleSpotlight(current)
		}},
		{Pattern: "/kudos", Page: "kudos.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			tab := r.URL.Query().Get("tab")
			if tab == "" {
				tab = "received"
			}
			return sampleKudos(current, tab)
		}},
		{Pattern: "/polls", Page: "polls.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return samplePolls(current)
		}},
		{Pattern: "/badges", Page: "badges.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleBadges(current)
		}},
		{Pattern: "/admin/dashboard", Page: "admin_dashboard.html", ExecuteName: "base", DefaultRole: "admin", Build: func(current fixtureUser, r *http.Request) any {
			return sampleAdminDashboard(current)
		}},
		{Pattern: "/admin/moderation", Page: "moderation_queue.html", ExecuteName: "base", DefaultRole: "admin", Build: func(current fixtureUser, r *http.Request) any {
			return sampleModeration(current)
		}},
		{Pattern: "/admin/foia", Page: "foia.html", ExecuteName: "base", DefaultRole: "admin", Build: func(current fixtureUser, r *http.Request) any {
			return sampleFOIA(current, r, false)
		}},
		{Pattern: "/admin/foia/search", Page: "foia.html", ExecuteName: "base", DefaultRole: "admin", Build: func(current fixtureUser, r *http.Request) any {
			return sampleFOIA(current, r, true)
		}},
		{Pattern: "/analytics/workforce", Page: "workforce_dashboard.html", ExecuteName: "base", DefaultRole: "manager", Build: func(current fixtureUser, r *http.Request) any {
			return sampleWorkforce(current)
		}},
		{Pattern: "/insights/network", Page: "network.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return sampleNetwork(current)
		}},
		{Pattern: "/orgchart", Page: "orgchart.html", ExecuteName: "base", DefaultRole: "admin", Build: func(current fixtureUser, r *http.Request) any {
			return sampleOrgchart(current)
		}},
		{Pattern: "/data-export", Page: "data_export.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return struct{ middleware.BaseData }{BaseData: current.baseData()}
		}},
		{Pattern: "/aup", Page: "aup.html", ExecuteName: "base", DefaultRole: "employee", Build: func(current fixtureUser, r *http.Request) any {
			return aupPage{BaseData: current.baseData(), Accepted: aupAccepted(r)}
		}},
	}
	for _, fixture := range fixtures {
		registerTemplateFixture(mux, pages, fixture)
	}

	mux.HandleFunc("GET /resumes", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "employee")
		if err := pages["resumes.html"].ExecuteTemplate(w, "base", sampleResumeIndex(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /workspaces", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/workspaces/workspace-1", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /departments", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/departments/department-1", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/groups/group-1", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /opportunities/search", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["posting_search.html"].ExecuteTemplate(w, "base", samplePostingSearch(current, r)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /opportunities/create", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["posting_create.html"].ExecuteTemplate(w, "base", samplePostingCreate(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /opportunities/create", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/opportunities/posting-1?role=manager", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /articles", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["article_view.html"].ExecuteTemplate(w, "base", sampleArticle(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /news", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["news_view.html"].ExecuteTemplate(w, "base", sampleNews(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /my-posts", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed?tab=postings", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /analytics/skills-gap", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/analytics/workforce", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /analytics/learning", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/analytics/workforce", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /insights/heatmap", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["heatmap.html"].ExecuteTemplate(w, "base", sampleHeatmap(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/users", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "admin")
		if err := pages["admin_users.html"].ExecuteTemplate(w, "base", sampleAdminUsers(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/audit", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "admin")
		if err := pages["admin_audit.html"].ExecuteTemplate(w, "base", sampleAdminAudit(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/report", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "admin")
		if err := pages["admin_report.html"].ExecuteTemplate(w, "report_base", sampleAdminReport(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/spotlight", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "admin")
		if err := pages["spotlight_create.html"].ExecuteTemplate(w, "base", sampleSpotlightCreate(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /admin/spotlight", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/spotlight?role=admin")
	})
	mux.HandleFunc("GET /admin/orgchart", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "admin")
		if err := pages["orgchart_assign.html"].ExecuteTemplate(w, "base", sampleOrgchartAssign(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/announcements/new", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/announcements?role=admin", http.StatusSeeOther)
	})

	mux.HandleFunc("GET /me/role", func(w http.ResponseWriter, r *http.Request) {
		current := selectedFixtureUser(r, "employee")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(current.Role))
	})
	mux.HandleFunc("GET /notifications/count", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleNotificationsCount(w, r)
	})
	mux.HandleFunc("GET /notifications/recent", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleNotificationsRecent(w, r)
	})
	mux.HandleFunc("POST /notifications/read-all", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleNotificationsReadAll(w, r)
	})
	mux.HandleFunc("POST /notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleNotificationRead(w, r)
	})
	mux.HandleFunc("POST /notifications/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleNotificationDelete(w, r)
	})
	mux.HandleFunc("GET /notifications/stream", func(w http.ResponseWriter, r *http.Request) {
		serveEventStream(w, r)
	})
	mux.HandleFunc("GET /messages/recent", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageRecent(w, r)
	})
	mux.HandleFunc("GET /messages/search-users", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageSearchUsers(w, r)
	})
	mux.HandleFunc("GET /messages/events", func(w http.ResponseWriter, r *http.Request) {
		serveEventStream(w, r)
	})
	mux.HandleFunc("POST /messages/start", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageStart(w, r)
	})
	mux.HandleFunc("POST /messages/new/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageNew(w, r)
	})
	mux.HandleFunc("GET /messages/chat/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageChatGet(w, r)
	})
	mux.HandleFunc("POST /messages/chat/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageChatPost(w, r)
	})
	mux.HandleFunc("POST /messages/chat/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessageAddMember(w, r)
	})
	mux.HandleFunc("POST /messages-typing/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /messages-pin/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMessagePin(w, r)
	})
	mux.HandleFunc("GET /feed/events", func(w http.ResponseWriter, r *http.Request) {
		serveEventStream(w, r)
	})
	mux.HandleFunc("POST /feed", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed")
	})
	mux.HandleFunc("POST /feed/{id}/like", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /feed/{id}/comment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /feed/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed")
	})
	mux.HandleFunc("POST /feed/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed")
	})
	mux.HandleFunc("POST /feed/drafts/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed/drafts")
	})
	mux.HandleFunc("GET /feed/drafts/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /feed/drafts/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed/drafts")
	})
	mux.HandleFunc("GET /feed/drafts/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed/drafts", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /profile/posts/{id}/pin", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/posts/{id}/unpin", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/avatar", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/experience/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/education/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/skills/{id}/endorse", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/mentor-1")
	})
	mux.HandleFunc("POST /profile/skills/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/mentor-1")
	})
	mux.HandleFunc("POST /connections/request/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleConnectionRequest(w, r)
	})
	mux.HandleFunc("POST /connections/accept/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleConnectionAccept(w, r)
	})
	mux.HandleFunc("POST /connections/reject/{id}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleConnectionReject(w, r)
	})
	mux.HandleFunc("POST /follow/{id}", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/mentor-1")
	})
	mux.HandleFunc("POST /feedback-ask/{userID}", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feedback-ask/mentor-1")
	})
	mux.HandleFunc("POST /onboarding/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/onboarding")
	})
	mux.HandleFunc("POST /certifications", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/certifications")
	})
	mux.HandleFunc("POST /certifications/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/certifications")
	})
	mux.HandleFunc("POST /accomplishments/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/accomplishments")
	})
	mux.HandleFunc("POST /notifications/preferences", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/notifications/preferences?saved=1")
	})
	mux.HandleFunc("POST /settings/matching-mode", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed")
	})
	mux.HandleFunc("GET /settings/matching-mode", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /settings/location-types", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/feed")
	})
	mux.HandleFunc("GET /settings/location-types", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /settings/sessions/revoke", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/settings/sessions?revoked=1")
	})
	mux.HandleFunc("POST /resumes/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/resumes")
	})
	mux.HandleFunc("GET /resumes/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/resumes", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /workspaces/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/workspaces/"+r.PathValue("id"))
	})
	mux.HandleFunc("POST /workspaces/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/workspaces/"+r.PathValue("id"))
	})
	mux.HandleFunc("GET /workspaces/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/workspaces/"+r.PathValue("id"), http.StatusSeeOther)
	})
	mux.HandleFunc("GET /workspaces/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/workspaces/"+r.PathValue("id"), http.StatusSeeOther)
	})
	mux.HandleFunc("POST /profile/edit", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/experience/add", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/experience/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/education/add", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/education/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/skills/add", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("POST /profile/skills/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/profile/me")
	})
	mux.HandleFunc("GET /aup/accept", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/aup", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /bookmarks/add", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleBookmarkAdd(w, r)
	})
	mux.HandleFunc("POST /bookmarks/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleBookmarkDelete(w, r)
	})
	mux.HandleFunc("POST /groups/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleGroupJoin(w, r)
	})
	mux.HandleFunc("POST /groups/{id}/leave", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleGroupLeave(w, r)
	})
	mux.HandleFunc("POST /groups/{id}/post", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleGroupPost(w, r)
	})
	mux.HandleFunc("POST /mentors/request/{userID}", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMentorRequest(w, r)
	})
	mux.HandleFunc("POST /mentorship/{id}/accept", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMentorshipAccept(w, r)
	})
	mux.HandleFunc("POST /mentorship/{id}/decline", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMentorshipDecline(w, r)
	})
	mux.HandleFunc("POST /mentorship/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleMentorshipComplete(w, r)
	})
	mux.HandleFunc("POST /workspaces/{id}/notes", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleWorkspaceNote(w, r)
	})
	mux.HandleFunc("POST /workspaces/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleWorkspaceMember(w, r)
	})
	mux.HandleFunc("POST /workspaces/{id}/meetings", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/workspaces/"+r.PathValue("id"))
	})
	mux.HandleFunc("POST /news/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/news/"+r.PathValue("id"))
	})
	mux.HandleFunc("POST /admin/announcements/{id}/pin", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/announcements?role=admin")
	})
	mux.HandleFunc("POST /admin/announcements/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/announcements?role=admin")
	})
	mux.HandleFunc("POST /polls/{id}/vote", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handlePollVote(w, r)
	})
	mux.HandleFunc("POST /opportunities/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/opportunities/"+r.PathValue("id"))
	})
	mux.HandleFunc("GET /opportunities/{id}/apply", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/opportunities/"+r.PathValue("id"))
	})
	mux.HandleFunc("GET /opportunities/{id}/applications", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["posting_applications.html"].ExecuteTemplate(w, "base", samplePostingApplications(current, r.PathValue("id"))); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /opportunities/{id}/outcome", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "manager")
		if err := pages["posting_outcome.html"].ExecuteTemplate(w, "base", samplePostingOutcome(current, r.PathValue("id"))); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /admin/moderation/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleModerationResolve(w, r)
	})
	mux.HandleFunc("GET /profile/edit", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "employee")
		if err := pages["profile_edit.html"].ExecuteTemplate(w, "base", sampleProfileEdit(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /accomplishments/add", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "employee")
		if err := pages["accomplishment_form.html"].ExecuteTemplate(w, "base", sampleAccomplishmentForm(current)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /accomplishments/add", func(w http.ResponseWriter, r *http.Request) {
		redirectBack(w, r, "/accomplishments")
	})
	mux.HandleFunc("GET /analytics/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, "employee")
		if err := pages["leaderboard.html"].ExecuteTemplate(w, "base", sampleLeaderboard(current, r)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /admin/foia/export", func(w http.ResponseWriter, r *http.Request) {
		workflowFixtures.handleFOIAExport(w, r)
	})
	mux.HandleFunc("GET /data-export/download", func(w http.ResponseWriter, r *http.Request) {
		current := selectedFixtureUser(r, "employee")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="job-portal-data.json"`)
		writeJSON(w, map[string]any{
			"profile": map[string]string{
				"id":         current.ID,
				"first_name": current.FirstName,
				"last_name":  current.LastName,
				"headline":   current.Headline,
			},
			"connections": []string{"mentor-1", "coworker-1"},
		})
	})
	mux.HandleFunc("POST /aup/accept", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: aupCookieName, Value: "1", Path: "/", SameSite: http.SameSiteLaxMode})
		redirectBack(w, r, "/aup")
	})

	log.Printf("frontend test server listening on http://127.0.0.1:%s", addr)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+addr, mux))
}

const roleCookieName = "fixture_role"
const aupCookieName = "fixture_aup_accepted"

func registerTemplateFixture(mux *http.ServeMux, pages map[string]*template.Template, fixture templateFixture) {
	mux.HandleFunc("GET "+fixture.Pattern, func(w http.ResponseWriter, r *http.Request) {
		current := selectFixtureUser(w, r, fixture.DefaultRole)
		if err := pages[fixture.Page].ExecuteTemplate(w, fixture.ExecuteName, fixture.Build(current, r)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func parseTemplate(name string) *template.Template {
	return template.Must(template.ParseFS(
		os.DirFS("templates"),
		"layouts/*.html",
		name,
	))
}

func selectedFixtureUser(r *http.Request, defaultRole string) fixtureUser {
	role := defaultRole
	if queryRole := strings.ToLower(r.URL.Query().Get("role")); queryRole != "" {
		role = queryRole
	} else if cookie, err := r.Cookie(roleCookieName); err == nil {
		role = strings.ToLower(cookie.Value)
	}
	user, ok := fixtureUsers[role]
	if !ok {
		return fixtureUsers[defaultRole]
	}
	return user
}

func selectFixtureUser(w http.ResponseWriter, r *http.Request, defaultRole string) fixtureUser {
	current := selectedFixtureUser(r, defaultRole)
	setRoleCookie(w, current.Role)
	return current
}

func setRoleCookie(w http.ResponseWriter, role string) {
	http.SetCookie(w, &http.Cookie{Name: roleCookieName, Value: role, Path: "/", SameSite: http.SameSiteLaxMode})
}

func (u fixtureUser) baseData() middleware.BaseData {
	return middleware.BaseData{
		UserID: u.ID,
		Nav: middleware.UserInfo{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Initials:  string(u.FirstName[0]) + string(u.LastName[0]),
			Role:      u.Role,
			AvatarURL: u.AvatarURL,
		},
	}
}

func sampleFeedPage(current fixtureUser, tab string) feedPage {
	return feedPage{
		BaseData: current.baseData(),
		Tab:      tab,
		Posts: []feedPost{{
			ID:              "abcde-1234",
			AuthorID:        "mentor-1",
			AuthorFirstName: "Morgan",
			AuthorLastName:  "Lee",
			AuthorHeadline:  "Program Analyst",
			RenderedContent: template.HTML("Welcome to the shared feed fixture."),
			LikeCount:       3,
			CommentCount:    1,
			Comments: []feedComment{{
				AuthorID:        "coworker-1",
				AuthorFirstName: "Alex",
				AuthorLastName:  "Nguyen",
				RenderedContent: template.HTML("Thanks for posting this update."),
			}},
		}},
		Postings: []feedPosting{{
			ID:          "posting-1",
			Type:        "detail",
			AuthorName:  "Taylor Jordan",
			Title:       "Temporary detail opportunity",
			Department:  "NRCS",
			Location:    "Washington, DC",
			Description: "Support the USDA workforce dashboard refresh.",
			CreatedAt:   fixtureTime{label: "May 23, 2026"},
		}},
	}
}

func sampleFeedDraftPage(current fixtureUser) feedDraftPage {
	return feedDraftPage{
		BaseData: current.baseData(),
		Drafts: []feedDraft{
			{ID: "draft-1", Content: "**Weekly update**\n\n- Completed onboarding check-ins\n- Published staffing notes", RenderedContent: template.HTML("<strong>Weekly update</strong><br><br>&bull; Completed onboarding check-ins<br>&bull; Published staffing notes"), CreatedAt: fixtureNow.AddDate(0, 0, -2)},
			{ID: "draft-2", Content: "Need review on detail interview templates before Friday.", RenderedContent: template.HTML("Need review on detail interview templates before Friday."), CreatedAt: fixtureNow.AddDate(0, 0, -1)},
		},
		ScheduledPosts: []scheduledFeedPost{
			{ID: "post-future-1", Content: "Reminder: Q3 networking event starts Monday.", RenderedContent: template.HTML("Reminder: Q3 networking event starts Monday."), CreatedAt: fixtureNow.AddDate(0, 0, -3), ScheduledAt: fixtureNow.AddDate(0, 0, 2).Add(3 * time.Hour)},
		},
	}
}

func sampleProfileView(current fixtureUser, profileID string) userpkg.ProfileView {
	return workflowFixtures.profileView(current, profileID)
}

func sampleProfileEdit(current fixtureUser) userpkg.ProfileData {
	birthday := fixtureNow.AddDate(-34, 0, 0)
	hireDate := fixtureNow.AddDate(-6, 0, 0)
	until := fixtureNow.AddDate(0, 0, 14)
	return userpkg.ProfileData{
		BaseData: current.baseData(),
		User: userpkg.User{
			ID:                current.ID,
			Email:             strings.ToLower(current.FirstName) + "@usda.gov",
			FirstName:         current.FirstName,
			LastName:          current.LastName,
			Headline:          current.Headline,
			About:             current.About,
			Location:          current.Location,
			Birthday:          &birthday,
			HireDate:          &hireDate,
			WorkStatus:        "telework",
			WorkStatusUntil:   &until,
			ProfileVisibility: "connections",
		},
		IsOwnProfile: true,
	}
}

func sampleAccomplishmentForm(current fixtureUser) accomplishmentpkg.FormPage {
	return accomplishmentpkg.FormPage{
		BaseData:       current.baseData(),
		Accomplishment: nil,
		Error:          "",
	}
}

func samplePostingSearch(current fixtureUser, r *http.Request) postingpkg.SearchPage {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		query = "analytics"
	}
	selectedType := strings.TrimSpace(r.URL.Query().Get("type"))
	postings := []postingpkg.Posting{
		{
			ID:          "posting-1",
			AuthorID:    "manager-1",
			AuthorName:  "Taylor Jordan",
			Title:       "Temporary detail opportunity",
			Description: "Support the USDA workforce dashboard refresh.",
			Type:        "detail",
			Department:  "NRCS",
			Location:    "Washington, DC",
			Status:      "active",
			CreatedAt:   fixtureNow.AddDate(0, 0, -2),
		},
		{
			ID:          "posting-2",
			AuthorID:    "manager-1",
			AuthorName:  "Taylor Jordan",
			Title:       "Data quality project sprint",
			Description: "Improve cross-agency data quality checks.",
			Type:        "project",
			Department:  "Forest Service",
			Location:    "Remote",
			Status:      "active",
			CreatedAt:   fixtureNow.AddDate(0, 0, -5),
		},
	}
	if selectedType == "project" {
		postings = postings[1:]
	} else if selectedType == "detail" {
		postings = postings[:1]
	}
	return postingpkg.SearchPage{
		BaseData: current.baseData(),
		Query:    query,
		Type:     selectedType,
		Postings: postings,
	}
}

func samplePostingCreate(current fixtureUser) postingpkg.CreatePage {
	return postingpkg.CreatePage{BaseData: current.baseData()}
}

func samplePostingApplications(current fixtureUser, postingID string) postingpkg.ApplicationsPage {
	if strings.TrimSpace(postingID) == "" {
		postingID = "posting-1"
	}
	return postingpkg.ApplicationsPage{
		BaseData: current.baseData(),
		Posting: postingpkg.Posting{
			ID:         postingID,
			Title:      "Temporary detail opportunity",
			Type:       "detail",
			Status:     "active",
			Department: "NRCS",
		},
		Applications: []postingpkg.Application{
			{
				ID:            "app-1",
				PostingID:     postingID,
				ApplicantID:   "employee-1",
				ApplicantName: "Riley Carter",
				CoverLetter:   "I can support this initiative immediately.",
				Status:        "pending",
				CreatedAt:     fixtureNow.AddDate(0, 0, -1),
			},
		},
	}
}

func samplePostingOutcome(current fixtureUser, postingID string) postingpkg.OutcomePage {
	if strings.TrimSpace(postingID) == "" {
		postingID = "posting-1"
	}
	return postingpkg.OutcomePage{
		BaseData: current.baseData(),
		Posting: postingpkg.Posting{
			ID:            postingID,
			Title:         "Temporary detail opportunity",
			Type:          "detail",
			Department:    "NRCS",
			Location:      "Washington, DC",
			OutcomeStatus: "completed",
			Outcome:       "Delivered dashboard rollout with documentation.",
			HasOutcome:    true,
		},
	}
}

func sampleAdminUsers(current fixtureUser) adminpkg.AdminPage {
	users := []adminpkg.User{
		{ID: "employee-1", Email: "riley@usda.gov", FirstName: "Riley", LastName: "Carter", Role: "employee", DepartmentID: "dept-1", DepartmentName: "NRCS"},
		{ID: "manager-1", Email: "taylor@usda.gov", FirstName: "Taylor", LastName: "Jordan", Role: "manager", DepartmentID: "dept-2", DepartmentName: "Forest Service"},
	}
	depts := []adminpkg.Department{
		{ID: "dept-1", Name: "NRCS"},
		{ID: "dept-2", Name: "Forest Service"},
		{ID: "dept-3", Name: "FSA"},
	}
	return adminpkg.AdminPage{BaseData: current.baseData(), Users: users, Departments: depts}
}

func sampleAdminAudit(current fixtureUser) adminpkg.AuditPage {
	return adminpkg.AuditPage{
		BaseData: current.baseData(),
		Entries: []adminpkg.AuditEntry{
			{ID: "audit-1", ActorName: "Sam Patel", Action: "role_change", TargetName: "Riley Carter", Details: "employee -> manager", CreatedAt: fixtureNow.Add(-2 * time.Hour)},
			{ID: "audit-2", ActorName: "Taylor Jordan", Action: "workspace_update", TargetName: "Workforce Strategy Circle", Details: "Updated meeting cadence and join guidance", CreatedAt: fixtureNow.Add(-26 * time.Hour)},
		},
	}
}

func sampleAdminReport(current fixtureUser) adminpkg.ReportPage {
	return adminpkg.ReportPage{
		GeneratedAt:      fixtureNow,
		HeadcountsByDept: []adminpkg.NameCount{{Name: "NRCS", Count: 86}, {Name: "Forest Service", Count: 74}, {Name: "FSA", Count: 52}},
		HeadcountsByRole: []adminpkg.RoleCount{{Role: "employee", Count: 220}, {Role: "manager", Count: 24}, {Role: "admin", Count: 4}},
		SkillsCoverage:   []adminpkg.NameCount{{Name: "Program Management", Count: 64}, {Name: "Analytics", Count: 48}, {Name: "GIS", Count: 31}},
		TotalUsers:       248,
		UsersWithSkills:  189,
		TotalPosts:       42,
		TotalConnections: 17,
	}
}

func sampleSpotlightCreate(current fixtureUser) spotlightpkg.CreatePage {
	users := []spotlightpkg.UserOption{
		{ID: "employee-1", Name: "Riley Carter"},
		{ID: "coworker-1", Name: "Alex Nguyen"},
		{ID: "diana-1", Name: "Diana Prince"},
	}
	return spotlightpkg.CreatePage{BaseData: current.baseData(), Users: users}
}

func sampleOrgchartAssign(current fixtureUser) orgchartpkg.AssignPage {
	users := []orgchartpkg.SelectUser{
		{ID: "employee-1", Name: "Riley Carter"},
		{ID: "manager-1", Name: "Taylor Jordan"},
		{ID: "mentor-1", Name: "Morgan Lee"},
	}
	return orgchartpkg.AssignPage{BaseData: current.baseData(), Users: users}
}

func sampleLeaderboard(current fixtureUser, r *http.Request) analyticspkg.LeaderboardPage {
	selected := strings.TrimSpace(r.URL.Query().Get("department"))
	options := []analyticspkg.DepartmentOption{
		{ID: "dept-1", Name: "NRCS"},
		{ID: "dept-2", Name: "Forest Service"},
	}
	entries := []analyticspkg.LeaderboardEntry{
		{Rank: 1, UserID: "mentor-1", Name: "Morgan Lee", Department: "NRCS", Score: 128},
		{Rank: 2, UserID: "employee-1", Name: "Riley Carter", Department: "Forest Service", Score: 117},
	}
	if selected == "dept-1" {
		entries = entries[:1]
	}
	if selected == "dept-2" {
		entries = entries[1:]
	}
	return analyticspkg.LeaderboardPage{
		BaseData:           current.baseData(),
		Entries:            entries,
		DepartmentOptions:  options,
		SelectedDepartment: selected,
	}
}

func sampleResumeData(current fixtureUser) any {
	return map[string]any{
		"UserID":    current.ID,
		"FirstName": current.FirstName,
		"LastName":  current.LastName,
		"Headline":  current.Headline,
		"Location":  current.Location,
		"About":     current.About,
		"Experiences": []resumeExperience{{
			Title:       "Branch Coordinator",
			Company:     "USDA",
			Location:    current.Location,
			Description: "Coordinated detail assignments and onboarding plans for workforce rotations.",
			StartDate:   time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndDate:     sql.NullTime{Time: time.Date(2024, time.August, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		}},
		"Educations": []resumeEducation{{
			School:       "University of Missouri",
			Degree:       "B.S.",
			FieldOfStudy: "Agricultural Economics",
			StartYear:    2014,
			EndYear:      sql.NullInt64{Int64: 2018, Valid: true},
		}},
		"Skills": []string{"Workforce Planning", "Program Management", "Data Analysis"},
	}
}

func sampleResumeIndex(current fixtureUser) resumepkg.ResumePage {
	return resumepkg.ResumePage{
		BaseData: current.baseData(),
		Resumes: []resumepkg.Resume{{
			ID:           "resume-1",
			OriginalName: "riley-carter-resume.pdf",
			ContentType:  "application/pdf",
			FileSize:     1024,
			CreatedAt:    "May 20, 2026",
		}},
	}
}

func sampleAccomplishments(current fixtureUser, period string) accomplishmentpkg.ListPage {
	return accomplishmentpkg.ListPage{
		BaseData:     current.baseData(),
		PeriodFilter: period,
		Accomplishments: []accomplishmentpkg.Accomplishment{{
			ID:          "acc-1",
			UserID:      current.ID,
			Title:       "Launched workforce mentoring dashboard",
			Description: "Delivered the first cross-agency mentorship dashboard with manager-ready participation summaries.",
			PeriodType:  period,
			PeriodStart: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			PeriodEnd:   time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
			CreatedAt:   fixtureNow.Add(-72 * time.Hour),
		}},
	}
}

func sampleCertifications(current fixtureUser) certificationpkg.CertificationsPage {
	return certificationpkg.CertificationsPage{
		BaseData: current.baseData(),
		Certifications: []certificationpkg.Certification{{
			ID:        "cert-1",
			Name:      "USDA Data Stewardship",
			Issuer:    "USDA University",
			IssuedOn:  sql.NullTime{Time: time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
			ExpiresOn: sql.NullTime{Time: time.Date(2027, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true},
			Status:    "Active",
		}},
	}
}

func sampleDigest(current fixtureUser) digestpkg.DigestPage {
	return digestpkg.DigestPage{
		BaseData:    current.baseData(),
		Connections: []digestpkg.Connection{{ID: "mentor-1", Name: "Morgan Lee"}},
		PostEngagement: []digestpkg.PostEngagement{{
			ID:        "post-1",
			Content:   "Shared the new analytics playbook with regional teams.",
			Likes:     8,
			Comments:  2,
			CreatedAt: fixtureNow.Add(-24 * time.Hour),
		}},
		Kudos:            []digestpkg.Kudo{{ID: "kudo-1", SenderName: "Alex Nguyen", Message: "Thanks for helping with onboarding week.", CreatedAt: fixtureNow.Add(-48 * time.Hour)}},
		NotificationsNew: 3,
		Postings:         []digestpkg.MatchingPosting{{ID: "posting-1", Title: "Temporary detail opportunity", Department: "NRCS", Skill: "Program Management", CreatedAt: fixtureNow.Add(-12 * time.Hour)}},
		TotalLikes:       8,
		TotalComments:    2,
	}
}

func sampleFeedbackRequest(current fixtureUser, receiverID string) feedbackpkg.RequestPage {
	receiver := lookupUser(receiverID)
	return feedbackpkg.RequestPage{
		BaseData:     current.baseData(),
		ReceiverID:   receiver.ID,
		ReceiverName: receiver.FirstName + " " + receiver.LastName,
	}
}

func sampleSearch(current fixtureUser, query string) searchpkg.SearchPage {
	return workflowFixtures.searchPage(current, query)
}

func sampleOnboarding(current fixtureUser) onboardingpkg.OnboardingPage {
	completedAt := fixtureNow.Add(-72 * time.Hour)
	steps := []onboardingpkg.Step{{ID: "profile", Title: "Complete your profile", Description: "Add your role, location, and work status.", CompletedAt: &completedAt}, {ID: "connections", Title: "Connect with your team", Description: "Build a starter network for messaging and mentorship.", CompletedAt: nil}, {ID: "digest", Title: "Review the weekly digest", Description: "See recent activity, kudos, and matching opportunities.", CompletedAt: nil}}
	return onboardingpkg.OnboardingPage{BaseData: current.baseData(), Steps: steps, Completed: 1, Total: len(steps), Percent: 33}
}

func sampleConnections(current fixtureUser) connectionpkg.ConnectionsPage {
	return workflowFixtures.connectionsPage(current)
}

func sampleNotificationPreferences(current fixtureUser, saved bool) notificationPreferencesPage {
	return notificationPreferencesPage{
		BaseData: current.baseData(),
		Saved:    saved,
		Prefs: []notificationPreference{
			{Type: "mentions", Label: "Mentions and replies", Enabled: true},
			{Type: "messages", Label: "Direct messages", Enabled: true},
			{Type: "announcements", Label: "Announcements and portal updates", Enabled: true},
			{Type: "mentorship", Label: "Mentorship matches and reminders", Enabled: false},
			{Type: "digest", Label: "Weekly digest emails", Enabled: true},
		},
	}
}

func sampleSessions(current fixtureUser, revoked bool) sessionsPage {
	return sessionsPage{
		BaseData: current.baseData(),
		Revoked:  revoked,
		Sessions: []sessionFixture{
			{
				UserAgent: "Chrome on Windows 11",
				CreatedAt: "May 23, 2026 at 10:15 AM",
				MaskedID:  "...A91K",
				Token:     "session-current",
				IsCurrent: true,
			},
			{
				UserAgent: "Safari on iPhone",
				CreatedAt: "May 21, 2026 at 7:42 PM",
				MaskedID:  "...B52Q",
				Token:     "session-mobile",
			},
			{
				UserAgent: "Edge on USDA laptop",
				CreatedAt: "May 20, 2026 at 8:03 AM",
				MaskedID:  "...C77M",
				Token:     "session-laptop",
			},
		},
	}
}

func sampleBookmarks(current fixtureUser, tab string) bookmarkpkg.BookmarkPage {
	return workflowFixtures.bookmarksPage(current, tab)
}

func sampleGroup(current fixtureUser, groupID string) grouppkg.GroupPage {
	return workflowFixtures.groupPage(current, groupID)
}

func sampleMentorship(current fixtureUser, tab string) mentorshippkg.MentorPage {
	return workflowFixtures.mentorshipPage(current, tab)
}

func sampleDepartment(current fixtureUser) departmentpkg.DeptDetailPage {
	return departmentpkg.DeptDetailPage{
		BaseData:   current.baseData(),
		Department: departmentpkg.Department{ID: "department-1", Name: "Office of Workforce Strategy", Description: "Coordinates workforce planning, development, and internal mobility.", ParentID: "department-root", ParentName: "Office of the Chief Human Capital Officer"},
		Employees:  []departmentpkg.Employee{{ID: current.ID, FirstName: current.FirstName, LastName: current.LastName, Headline: current.Headline}, {ID: "mentor-1", FirstName: "Morgan", LastName: "Lee", Headline: "Program Analyst"}},
	}
}

func sampleWorkspace(current fixtureUser) workspacepkg.ViewPage {
	return workflowFixtures.workspacePage(current)
}

func samplePosting(current fixtureUser) postingpkg.PostingPage {
	return workflowFixtures.postingPage(current)
}

func sampleArticle(current fixtureUser) articlepkg.ViewPage {
	return workflowFixtures.articlePage(current)
}

func sampleNews(current fixtureUser) newspkg.ArticlePage {
	author := lookupUser("coworker-2")
	return newspkg.ArticlePage{
		BaseData: current.baseData(),
		Article:  newspkg.NewsArticle{ID: "news-1", AuthorID: author.ID, AuthorName: author.FirstName + " " + author.LastName, Title: "USDA launches workforce planning sprint", Content: "Regional offices can now review the latest workforce planning sprint materials in the shared workspace.", Published: false, CreatedAt: fixtureNow.AddDate(0, 0, -2)},
		IsAuthor: current.ID == author.ID || current.Role == "admin",
	}
}

func sampleAnnouncements(current fixtureUser) announcementpkg.IndexPage {
	return announcementpkg.IndexPage{
		BaseData:      current.baseData(),
		Announcements: []announcementpkg.Announcement{{ID: "announcement-1", Title: "Workforce dashboard pilot opens Monday", Body: "Managers can now review staffing trend summaries and export snapshots for team meetings.", AuthorName: "USDA Comms", IsPinned: true, CreatedAt: fixtureNow.AddDate(0, 0, -1)}},
	}
}

func sampleSpotlight(current fixtureUser) spotlightpkg.IndexPage {
	diana := lookupUser("diana-1")
	currentSpotlight := &spotlightpkg.Spotlight{ID: "spotlight-1", UserID: diana.ID, UserName: diana.FirstName + " " + diana.LastName, Headline: diana.Headline, AvatarURL: diana.AvatarURL, WeekOf: fixtureNow.AddDate(0, 0, -7), Reason: "Recognized for leading the rotational opportunity rollout across multiple mission areas."}
	return spotlightpkg.IndexPage{BaseData: current.baseData(), Current: currentSpotlight, Past: []spotlightpkg.Spotlight{{ID: "spotlight-0", UserID: "coworker-1", UserName: "Alex Nguyen", WeekOf: fixtureNow.AddDate(0, 0, -14)}}}
}

func sampleKudos(current fixtureUser, tab string) kudospkg.KudosPage {
	return kudospkg.KudosPage{
		BaseData: current.baseData(),
		Tab:      tab,
		Kudos: []kudospkg.Kudo{{
			ID:           "kudo-1",
			SenderID:     "mentor-1",
			SenderName:   "Morgan Lee",
			ReceiverID:   current.ID,
			ReceiverName: current.FirstName + " " + current.LastName,
			Message:      "Thanks for keeping the workspace notes organized for the sprint.",
			CreatedAt:    fixtureNow.AddDate(0, 0, -2),
		}},
	}
}

func samplePolls(current fixtureUser) pollpkg.PollPage {
	return workflowFixtures.pollPage(current)
}

func sampleBadges(current fixtureUser) badgepkg.BadgePage {
	return badgepkg.BadgePage{
		BaseData:  current.baseData(),
		Earned:    []badgepkg.UserBadge{{Badge: badgepkg.Badge{ID: "badge-1", Name: "Connector", Description: "Built a strong internal network.", Icon: "🤝"}, EarnedAt: fixtureNow.AddDate(0, -1, 0)}},
		Available: []badgepkg.Badge{{ID: "badge-2", Name: "Mentor", Description: "Guided a colleague through a mentorship cycle.", Icon: "🌱"}},
	}
}

func sampleAdminDashboard(current fixtureUser) adminpkg.DashboardPage {
	return adminpkg.DashboardPage{
		BaseData:               current.baseData(),
		TotalUsers:             248,
		UsersByRole:            []adminpkg.RoleCount{{Role: "employee", Count: 220}, {Role: "manager", Count: 24}, {Role: "admin", Count: 4}},
		TotalDepartments:       18,
		PostsThisWeek:          42,
		NewConnectionsThisWeek: 17,
		ActivePostings:         9,
		PendingApplications:    13,
		RecentSignups:          []adminpkg.Signup{{ID: "employee-2", Email: "new.hire@usda.gov", FirstName: "New", LastName: "Hire", Role: "employee", CreatedAt: fixtureNow.AddDate(0, 0, -1)}},
		RecentAudit:            []adminpkg.AuditEntry{{ID: "audit-1", ActorName: "Sam Patel", Action: "role_change", Details: "employee -> manager", CreatedAt: fixtureNow.Add(-2 * time.Hour)}},
	}
}

func sampleModeration(current fixtureUser) moderationpkg.QueuePage {
	return workflowFixtures.moderationPage(current)
}

func sampleFOIA(current fixtureUser, r *http.Request, searched bool) foiapkg.SearchPage {
	return workflowFixtures.foiaPage(current, r, searched)
}

func sampleWorkforce(current fixtureUser) analyticspkg.WorkforcePage {
	return analyticspkg.WorkforcePage{
		BaseData:       current.baseData(),
		TotalEmployees: 248,
		Departments:    []analyticspkg.NamedCount{{Name: "NRCS", Count: 86}, {Name: "Forest Service", Count: 74}, {Name: "FSA", Count: 52}},
		Roles:          []analyticspkg.NamedCount{{Name: "employee", Count: 220}, {Name: "manager", Count: 24}, {Name: "admin", Count: 4}},
		TopSkills:      []analyticspkg.NamedCount{{Name: "Program Management", Count: 64}, {Name: "Analytics", Count: 48}, {Name: "GIS", Count: 31}},
		Engagement:     analyticspkg.EngagementTotals{Posts: 42, Comments: 96, Connections: 17, Kudos: 23},
	}
}

func sampleHeatmap(current fixtureUser) insightspkg.HeatmapPage {
	return insightspkg.HeatmapPage{
		BaseData:    current.baseData(),
		Departments: []string{"Office of Workforce Strategy", "Forest Service"},
		Skills:      []string{"Go", "Program Management", "GIS"},
		Rows: []insightspkg.HeatRow{
			{
				Department: "Office of Workforce Strategy",
				Cells: []insightspkg.HeatCell{
					{Count: 4, Color: template.CSS("rgb(46,125,50)")},
					{Count: 2, Color: template.CSS("rgb(150,200,150)")},
					{Count: 1, Color: template.CSS("rgb(200,230,200)")},
				},
			},
			{
				Department: "Forest Service",
				Cells: []insightspkg.HeatCell{
					{Count: 1, Color: template.CSS("rgb(200,230,200)")},
					{Count: 3, Color: template.CSS("rgb(95,165,95)")},
					{Count: 0, Color: template.CSS("#ffffff")},
				},
			},
		},
		MaxCount: 4,
	}
}

func sampleNetwork(current fixtureUser) insightspkg.NetworkPage {
	graph := template.JS(`{"nodes":[{"id":"` + current.ID + `","label":"` + current.FirstName + `","center":true},{"id":"mentor-1","label":"Morgan Lee"},{"id":"coworker-1","label":"Alex Nguyen"}],"edges":[{"source":"` + current.ID + `","target":"mentor-1"},{"source":"` + current.ID + `","target":"coworker-1"}]}`)
	return insightspkg.NetworkPage{BaseData: current.baseData(), GraphJSON: graph, ConnectionCount: 2}
}

func sampleOrgchart(current fixtureUser) orgchartpkg.ChartPage {
	managerNode := &orgchartpkg.Node{ID: "manager-1", Name: "Taylor Jordan", Headline: "Branch Chief", Department: "Office of Workforce Strategy"}
	managerNode.Children = []*orgchartpkg.Node{{ID: "employee-1", Name: "Riley Carter", Headline: "Soil Conservationist", Department: "Office of Workforce Strategy"}, {ID: "mentor-1", Name: "Morgan Lee", Headline: "Program Analyst", Department: "Office of Workforce Strategy"}}
	return orgchartpkg.ChartPage{BaseData: current.baseData(), Roots: []*orgchartpkg.Node{managerNode}}
}

func lookupUser(id string) fixtureUser {
	if user, ok := directoryUsers[id]; ok {
		return user
	}
	return directoryUsers["mentor-1"]
}

func aupAccepted(r *http.Request) bool {
	if r.URL.Query().Get("accepted") == "1" {
		return true
	}
	cookie, err := r.Cookie(aupCookieName)
	return err == nil && cookie.Value == "1"
}

func redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	if referer := r.Header.Get("Referer"); referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fallback, http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveEventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	<-r.Context().Done()
}
