package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/svgstat/svgstat/internal/config"
	"github.com/svgstat/svgstat/internal/database"
	"github.com/svgstat/svgstat/internal/project"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	repo := project.NewPostgresRepository(db.Pool)

	ctx := context.Background()

	now := time.Now()
	testProject := &project.Project{
		ID:                     "proj_test_123",
		ExternalProjectID:      "ext_test_123",
		TenantID:               "tenant_test",
		Slug:                   "my-awesome-project",
		Name:                   "My Awesome Project",
		Description:            "A test project for SVGStat",
		Status:                 "active",
		Visibility:             "public",
		WebsiteTrackingEnabled: true,
		WebsiteDomains:         []string{},
		RenderEnabled:          true,
		BadgeEnabled:           true,
		WidgetEnabled:          true,
		ChartEnabled:           false,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	err = repo.Create(ctx, testProject)
	if err != nil {
		log.Fatalf("Failed to create test project: %v", err)
	}

	log.Printf("Test project created successfully!")
	log.Printf("Slug: %s", testProject.Slug)
	log.Printf("Counter URL: http://localhost:8080/svg/%s/counter/visits.svg", testProject.Slug)
	log.Printf("Badge URL: http://localhost:8080/svg/%s/badge/downloads.svg", testProject.Slug)
}
