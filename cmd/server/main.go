package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bhyland-usda/job-portal/internal/auth"
	"github.com/bhyland-usda/job-portal/internal/database"
	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/user"
	"github.com/bhyland-usda/job-portal/internal/connection"
	"github.com/bhyland-usda/job-portal/internal/search"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions {
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()

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

	// Parse templates
	pages := map[string]*template.Template {
		"login.html":      parseTemplate("auth/login.html"),
		"register.html":   parseTemplate("auth/register.html"),
		"feed.html":       parseTemplate("feed/feed.html"),
		"profile.html":    parseTemplate("profile/view.html"),
		"profile_edit.html": parseTemplate("profile/edit.html"),
		"connections.html": parseTemplate("connections/index.html"),
		"search.html": parseTemplate("search/index.html"),
	}

	// Session manager
 	sessions := auth.NewSessionManager(redisClient, cfg.SessionSecret)

 	// Auth middleware
 	requireAuth := middleware.RequireAuth(sessions)

 	// Handlers
 	authHandler := auth.NewHandler(db, pages, sessions)
	connectionHandler := connection.NewHandler(db, pages)
	userHandler := user.NewHandler(db, pages, connectionHandler)
	searchHandler := search.NewHandler(db, pages, connectionHandler)

 	// Router
 	mux := http.NewServeMux()

 	mux.Handle("GET /static/", http.StripPrefix("/static/",
 		http.FileServer(http.Dir("static")),
	))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Routes
	authHandler.RegisterRoutes(mux)
	userHandler.RegisterRoutes(mux, requireAuth)
	connectionHandler.RegisterRoutes(mux, requireAuth)
	searchHandler.RegisterRoutes(mux, requireAuth)

	// Protected routes (TODO)
	mux.Handle("GET /feed", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		pages["feed.html"].ExecuteTemplate(w, "base", map[string]string {"UserID": userID})
	})))

	// Redirect root to feed
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		http.Redirect(w, r, "/feed", http.StatusSeeOther)
	})

	srv := &http.Server {
		Addr:         ":" + cfg.Port,
		Handler:      mux,
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
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

type config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	SessionSecret string
}

func loadConfig() config {
	return config {
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://jobportal:jobportal@localhost:5432/jobportal?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		SessionSecret: getEnv("SESSION_SECRET", "dev-secret-change-me"),
	}
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
