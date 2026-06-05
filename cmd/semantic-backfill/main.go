package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bhyland-usda/job-portal/internal/database"
	"github.com/bhyland-usda/job-portal/internal/semantic"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://jobportal:jobportal@localhost:5432/jobportal?sslmode=disable"
	}

	db, err := database.Connect(databaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	opportunityCount, err := semantic.BackfillOpportunityEmbeddings(ctx, db)
	if err != nil {
		log.Fatalf("failed to backfill opportunity embeddings: %v", err)
	}

	workspaceCount, err := semantic.BackfillWorkspaceEmbeddings(ctx, db)
	if err != nil {
		log.Fatalf("failed to backfill workspace embeddings: %v", err)
	}

	fmt.Printf("backfill complete: %d opportunities, %d workspaces\n", opportunityCount, workspaceCount)
}
