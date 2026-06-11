package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/bhyland-usda/job-portal/internal/accomplishment"
	"github.com/bhyland-usda/job-portal/internal/admin"
	"github.com/bhyland-usda/job-portal/internal/analytics"
	"github.com/bhyland-usda/job-portal/internal/announcement"
	"github.com/bhyland-usda/job-portal/internal/article"
	"github.com/bhyland-usda/job-portal/internal/aup"
	"github.com/bhyland-usda/job-portal/internal/auth"
	"github.com/bhyland-usda/job-portal/internal/badge"
	"github.com/bhyland-usda/job-portal/internal/bookmark"
	"github.com/bhyland-usda/job-portal/internal/certification"
	"github.com/bhyland-usda/job-portal/internal/connection"
	"github.com/bhyland-usda/job-portal/internal/database"
	"github.com/bhyland-usda/job-portal/internal/dataexport"
	"github.com/bhyland-usda/job-portal/internal/department"
	"github.com/bhyland-usda/job-portal/internal/digest"
	"github.com/bhyland-usda/job-portal/internal/feed"
	"github.com/bhyland-usda/job-portal/internal/feedback"
	"github.com/bhyland-usda/job-portal/internal/foia"
	"github.com/bhyland-usda/job-portal/internal/follow"
	"github.com/bhyland-usda/job-portal/internal/group"
	"github.com/bhyland-usda/job-portal/internal/insights"
	"github.com/bhyland-usda/job-portal/internal/kudos"
	"github.com/bhyland-usda/job-portal/internal/mentorship"
	"github.com/bhyland-usda/job-portal/internal/messaging"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/moderation"
	"github.com/bhyland-usda/job-portal/internal/news"
	"github.com/bhyland-usda/job-portal/internal/notification"
	"github.com/bhyland-usda/job-portal/internal/onboarding"
	"github.com/bhyland-usda/job-portal/internal/opportunity"
	"github.com/bhyland-usda/job-portal/internal/orgchart"
	"github.com/bhyland-usda/job-portal/internal/poll"
	"github.com/bhyland-usda/job-portal/internal/posts"
	"github.com/bhyland-usda/job-portal/internal/resume"
	"github.com/bhyland-usda/job-portal/internal/search"
	"github.com/bhyland-usda/job-portal/internal/semantic"
	"github.com/bhyland-usda/job-portal/internal/spotlight"
	"github.com/bhyland-usda/job-portal/internal/user"
	"github.com/bhyland-usda/job-portal/internal/verification"
	"github.com/bhyland-usda/job-portal/internal/workspace"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()
	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid runtime configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	if err := database.Migrate(db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	seedAdmin(db, cfg.AdminEmail, cfg.AdminPassword)

	// Parse templates
	pages := map[string]*template.Template{
		"login.html":                    parseTemplate("auth/login.html"),
		"register.html":                 parseTemplate("auth/register.html"),
		"feed.html":                     parseTemplate("feed/feed.html"),
		"profile.html":                  parseTemplate("profile/view.html"),
		"profile_edit.html":             parseTemplate("profile/edit.html"),
		"connections.html":              parseTemplate("connections/index.html"),
		"search.html":                   parseTemplate("search/index.html"),
		"experience_form.html":          parseTemplate("profile/experience_form.html"),
		"education_form.html":           parseTemplate("profile/education_form.html"),
		"skills_form.html":              parseTemplate("profile/skills_form.html"),
		"resumes.html":                  parseTemplate("resume/index.html"),
		"resume_view.html":              template.Must(template.ParseFiles("templates/resume/view.html")),
		"admin_users.html":              parseTemplate("admin/users.html"),
		"admin_audit.html":              parseTemplate("admin/audit.html"),
		"admin_semantic.html":           parseTemplate("admin/semantic.html"),
		"posting_create.html":           parseTemplate("opportunity/create.html"),
		"posting_view.html":             parseTemplate("opportunity/view.html"),
		"posting_matches.html":          parseTemplate("opportunity/matches.html"),
		"posting_edit.html":             parseTemplate("opportunity/edit.html"),
		"my_posts.html":                 parseTemplate("opportunity/my_posts.html"),
		"posting_search.html":           parseTemplate("opportunity/search.html"),
		"accomplishments.html":          parseTemplate("accomplishment/index.html"),
		"accomplishment_form.html":      parseTemplate("accomplishment/form.html"),
		"accomplishment_export.html":    template.Must(template.ParseFiles("templates/accomplishment/export.html")),
		"bookmarks.html":                parseTemplate("bookmark/index.html"),
		"kudos.html":                    parseTemplate("kudos/index.html"),
		"kudos_send.html":               parseTemplate("kudos/send.html"),
		"polls.html":                    parseTemplate("poll/index.html"),
		"poll_create.html":              parseTemplate("poll/create.html"),
		"badges.html":                   parseTemplate("badge/index.html"),
		"news.html":                     parseTemplate("news/index.html"),
		"news_view.html":                parseTemplate("news/view.html"),
		"news_create.html":              parseTemplate("news/create.html"),
		"spotlight.html":                parseTemplate("spotlight/index.html"),
		"spotlight_create.html":         parseTemplate("spotlight/create.html"),
		"articles.html":                 parseTemplate("article/index.html"),
		"article_view.html":             parseTemplate("article/view.html"),
		"article_form.html":             parseTemplate("article/form.html"),
		"hashtag.html":                  parseTemplate("feed/hashtag.html"),
		"departments.html":              parseTemplate("department/index.html"),
		"department_view.html":          parseTemplate("department/view.html"),
		"mentorship.html":               parseTemplate("mentorship/index.html"),
		"posting_apply.html":            parseTemplate("opportunity/apply.html"),
		"posting_applications.html":     parseTemplate("opportunity/applications.html"),
		"feedback.html":                 parseTemplate("feedback/index.html"),
		"feedback_request.html":         parseTemplate("feedback/request.html"),
		"feedback_respond.html":         parseTemplate("feedback/respond.html"),
		"groups.html":                   parseTemplate("group/index.html"),
		"group_view.html":               parseTemplate("group/view.html"),
		"group_create.html":             parseTemplate("group/create.html"),
		"onboarding.html":               parseTemplate("onboarding/index.html"),
		"drafts.html":                   parseTemplate("feed/drafts.html"),
		"trending.html":                 parseTemplate("feed/trending.html"),
		"skills_gap.html":               parseTemplate("analytics/skills_gap.html"),
		"learning.html":                 parseTemplate("analytics/learning.html"),
		"foia.html":                     parseTemplate("admin/foia.html"),
		"landing.html":                  template.Must(template.ParseFiles("templates/landing.html")),
		"workforce_dashboard.html":      parseTemplate("analytics/workforce_dashboard.html"),
		"leaderboard.html":              parseTemplate("analytics/leaderboard.html"),
		"heatmap.html":                  parseTemplate("insights/heatmap.html"),
		"network.html":                  parseTemplate("insights/network.html"),
		"admin_dashboard.html":          parseTemplate("admin/dashboard.html"),
		"admin_report.html":             template.Must(template.ParseFiles("templates/admin/report.html")),
		"announcements.html":            parseTemplate("announcement/index.html"),
		"announcement_create.html":      parseTemplate("announcement/create.html"),
		"posting_outcome.html":          parseTemplate("opportunity/outcome.html"),
		"posting_history.html":          parseTemplate("opportunity/history.html"),
		"digest.html":                   parseTemplate("digest/index.html"),
		"profile_print.html":            template.Must(template.ParseFiles("templates/profile/print.html")),
		"orgchart.html":                 parseTemplate("orgchart/index.html"),
		"orgchart_assign.html":          parseTemplate("orgchart/assign.html"),
		"certifications.html":           parseTemplate("certification/index.html"),
		"workspaces.html":               parseTemplate("workspace/index.html"),
		"workspace_view.html":           parseTemplate("workspace/view.html"),
		"notification_preferences.html": parseTemplate("notification/preferences.html"),
		"moderation_queue.html":         parseTemplate("moderation/queue.html"),
		"data_export.html":              parseTemplate("dataexport/index.html"),
		"sessions.html":                 parseTemplate("auth/sessions.html"),
		"aup.html":                      parseTemplate("aup/index.html"),
	}

	// Session manager
	sessions := auth.NewSessionManager(redisClient, cfg.SessionSecret)

	errorPages := map[int]*template.Template{
		http.StatusForbidden:           template.Must(template.ParseFiles("templates/errors/403.html")),
		http.StatusNotFound:            template.Must(template.ParseFiles("templates/errors/404.html")),
		http.StatusInternalServerError: template.Must(template.ParseFiles("templates/errors/500.html")),
	}

	// Auth middleware. requireAuth also enforces the Acceptable Use Policy gate:
	// once auth populates the user context, RequireAUP redirects users who haven't
	// accepted the AUP (it self-exempts /aup, /aup/accept, /logout, /static).
	baseAuth := middleware.RequireAuthWithDB(sessions, db)
	aupGate := middleware.RequireAUP(db)
	csrfGate := middleware.RequireSameOriginUnsafeMethods
	requireAuth := func(next http.Handler) http.Handler { return baseAuth(aupGate(csrfGate(next))) }
	requireAdmin := middleware.RequireRole(db, "admin")

	// Handlers
	authHandler := auth.NewHandler(db, pages, sessions)
	notificationHandler := notification.NewHandler(db, pages)
	connectionHandler := connection.NewHandler(db, pages, notificationHandler)
	userHandler := user.NewHandler(db, pages, connectionHandler)
	searchHandler := search.NewHandler(db, pages, connectionHandler)
	feedHandler := feed.NewHandler(db, pages, notificationHandler)
	messagingHandler := messaging.NewHandler(db, pages, notificationHandler)
	resumeHandler := resume.NewHandler(db, pages)
	adminHandler := admin.NewHandler(db, pages)
	opportunityHandler := opportunity.NewHandler(db, pages)
	opportunityHandler.SetNotificationHandler(notificationHandler)
	postsHandler := posts.NewHandler(db, pages)
	accomplishmentHandler := accomplishment.NewHandler(db, pages)
	bookmarkHandler := bookmark.NewHandler(db, pages)
	kudosHandler := kudos.NewHandler(db, pages, notificationHandler)
	pollHandler := poll.NewHandler(db, pages)
	badgeHandler := badge.NewHandler(db, pages)
	newsHandler := news.NewHandler(db, pages)
	spotlightHandler := spotlight.NewHandler(db, pages)
	articleHandler := article.NewHandler(db, pages)
	departmentHandler := department.NewHandler(db, pages)
	mentorshipHandler := mentorship.NewHandler(db, pages, notificationHandler)
	feedbackHandler := feedback.NewHandler(db, pages, notificationHandler)
	followHandler := follow.NewHandler(db)
	verificationHandler := verification.NewHandler(db)
	foiaHandler := foia.NewHandler(db, pages)
	groupHandler := group.NewHandler(db, pages)
	onboardingHandler := onboarding.NewHandler(db, pages)
	analyticsHandler := analytics.NewHandler(db, pages)
	insightsHandler := insights.NewHandler(db, pages)
	announcementHandler := announcement.NewHandler(db, pages)
	digestHandler := digest.NewHandler(db, pages)
	orgchartHandler := orgchart.NewHandler(db, pages)
	certificationHandler := certification.NewHandler(db, pages)
	workspaceHandler := workspace.NewHandler(db, pages)
	moderationHandler := moderation.NewHandler(db, pages)
	dataExportHandler := dataexport.NewHandler(db, pages)
	aupHandler := aup.NewHandler(db, pages)
	requireManager := middleware.RequireRole(db, "manager", "admin")

	// Router
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static")),
	))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("GET /avatar/{id}", requireAuth(http.HandlerFunc(userHandler.ServeAvatar)))

	// Routes
	authHandler.RegisterRoutes(mux)
	notificationHandler.RegisterRoutes(mux, requireAuth)
	userHandler.RegisterRoutes(mux, requireAuth)
	connectionHandler.RegisterRoutes(mux, requireAuth)
	searchHandler.RegisterRoutes(mux, requireAuth)
	feedHandler.RegisterRoutes(mux, requireAuth)
	messagingHandler.RegisterRoutes(mux, requireAuth)
	resumeHandler.RegisterRoutes(mux, requireAuth)
	adminHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	opportunityHandler.RegisterRoutes(mux, requireAuth, requireManager)
	postsHandler.RegisterRoutes(mux, requireAuth)
	accomplishmentHandler.RegisterRoutes(mux, requireAuth)
	bookmarkHandler.RegisterRoutes(mux, requireAuth)
	kudosHandler.RegisterRoutes(mux, requireAuth)
	pollHandler.RegisterRoutes(mux, requireAuth, requireManager)
	badgeHandler.RegisterRoutes(mux, requireAuth)
	newsHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	spotlightHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	articleHandler.RegisterRoutes(mux, requireAuth)
	departmentHandler.RegisterRoutes(mux, requireAuth)
	mentorshipHandler.RegisterRoutes(mux, requireAuth)
	feedbackHandler.RegisterRoutes(mux, requireAuth)
	followHandler.RegisterRoutes(mux, requireAuth)
	verificationHandler.RegisterRoutes(mux, requireAuth)
	foiaHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	groupHandler.RegisterRoutes(mux, requireAuth)
	onboardingHandler.RegisterRoutes(mux, requireAuth)
	analyticsHandler.RegisterRoutes(mux, requireAuth, requireManager)
	insightsHandler.RegisterRoutes(mux, requireAuth, requireManager)
	announcementHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	digestHandler.RegisterRoutes(mux, requireAuth)
	orgchartHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	certificationHandler.RegisterRoutes(mux, requireAuth)
	workspaceHandler.RegisterRoutes(mux, requireAuth)
	moderationHandler.RegisterRoutes(mux, requireAuth, requireAdmin)
	dataExportHandler.RegisterRoutes(mux, requireAuth)
	aupHandler.RegisterRoutes(mux, requireAuth)
	mux.Handle("POST /settings/matching-mode", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		mode := strings.ToLower(strings.TrimSpace(r.FormValue("mode")))
		enabled := mode == "on" || mode == "1" || mode == "true"
		semantic.SetPreferenceCookie(w, r, enabled)

		returnTo := strings.TrimSpace(r.FormValue("return_to"))
		if returnTo == "" {
			returnTo = r.Header.Get("Referer")
		}
		returnTo = middleware.SafeRedirectTarget(returnTo, "/feed")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
	})))
	mux.Handle("GET /settings/matching-mode", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})))
	mux.Handle("POST /settings/location-types", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		pref := semantic.LocationTypePreference{
			Remote: r.FormValue("remote") != "",
			Hybrid: r.FormValue("hybrid") != "",
			Onsite: r.FormValue("onsite") != "",
		}
		if len(pref.Selected()) == 0 {
			pref = semantic.DefaultLocationTypePreference()
		}
		semantic.SetLocationTypePreferenceCookie(w, r, pref)

		returnTo := strings.TrimSpace(r.FormValue("return_to"))
		if returnTo == "" {
			returnTo = r.Header.Get("Referer")
		}
		returnTo = middleware.SafeRedirectTarget(returnTo, "/feed")
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
	})))
	mux.Handle("GET /settings/location-types", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})))

	// Root: landing page for guests, redirect to feed for logged-in users
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if userID, err := sessions.GetUserID(r.Context(), r); err == nil && userID != "" {
			http.Redirect(w, r, "/feed", http.StatusSeeOther)
			return
		}

		pages["landing.html"].ExecuteTemplate(w, "landing", nil)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      middleware.WithErrorPages(middleware.WithSecurityHeaders(middleware.RequireCSRFTokens(mux)), errorPages),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

type config struct {
	Port          string
	AppEnv        string
	DatabaseURL   string
	RedisURL      string
	SessionSecret string
	AdminEmail    string
	AdminPassword string
}

func loadConfig() config {
	return config{
		Port:          getEnv("PORT", "8080"),
		AppEnv:        strings.ToLower(getEnv("APP_ENV", "development")),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://jobportal:jobportal@localhost:5432/jobportal?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		SessionSecret: getEnv("SESSION_SECRET", "dev-secret-change-me"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@jobportal.local"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "changeme123"),
	}
}

func validateConfig(cfg config) error {
	if !isProductionLike(cfg.AppEnv) {
		return nil
	}

	if weakSecret(cfg.SessionSecret) {
		return fmt.Errorf("SESSION_SECRET is weak or default-like for %s", cfg.AppEnv)
	}
	if weakAdminPassword(cfg.AdminPassword) {
		return fmt.Errorf("ADMIN_PASSWORD is weak or default-like for %s", cfg.AppEnv)
	}
	if strings.Contains(strings.ToLower(cfg.DatabaseURL), "sslmode=disable") {
		return fmt.Errorf("DATABASE_URL must not disable TLS for %s", cfg.AppEnv)
	}

	return nil
}

func isProductionLike(env string) bool {
	env = strings.ToLower(strings.TrimSpace(env))
	return env == "production" || env == "prod" || env == "staging"
}

func weakSecret(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if len(v) < 32 {
		return true
	}
	return v == "dev-secret-change-me" || v == "change-me-in-production"
}

func weakAdminPassword(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if len(v) < 12 {
		return true
	}
	return v == "changeme123" || v == "password" || v == "admin" || v == "admin123"
}

func getEnv(key, fallback string) string {
	if v, _ := os.LookupEnv(key); v != "" {
		return v
	}

	return fallback
}

// HELPER FUNCTIONS
func parseTemplate(name string) *template.Template {
	return template.Must(template.ParseFS(
		os.DirFS("templates"),
		"layouts/*.html",
		name,
	))
}

func seedAdmin(db *sql.DB, email, password string) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	if err != nil {
		slog.Error("failed to check for admin", "error", err)
		return
	}

	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash admin password", "error", err)
		return
	}

	_, err = db.Exec(
		`INSERT INTO users (id, email, password_hash, first_name, last_name, role)
		 VALUES (gen_random_uuid(), $1, $2, 'System', 'Admin', 'admin')
		 ON CONFLICT (email) DO NOTHING`,
		email, string(hash),
	)
	if err != nil {
		slog.Error("failed to seed admin", "error", err)
		return
	}

	slog.Info("seeded default admin account", "email", email)
}
