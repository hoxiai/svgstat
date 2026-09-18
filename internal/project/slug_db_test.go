package project

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/database"
	"github.com/joho/godotenv"
)

func TestDeletedProjectSlugCanBeReused(t *testing.T) {
	_ = godotenv.Load("../../.env")
	db, err := database.New(config.Load())
	if err != nil {
		t.Skipf("Skipping test: PostgreSQL not reachable: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := NewPostgresRepository(db.Pool)
	suffix := time.Now().UnixNano()
	userID := fmt.Sprintf("slug-test-user-%d", suffix)
	if _, err := db.Pool.Exec(ctx, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'x')`, userID, userID+"@example.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer func() {
		_, _ = db.Pool.Exec(ctx, `DELETE FROM projects WHERE user_id = $1`, userID)
		_, _ = db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	}()

	slug := fmt.Sprintf("slug-reuse-%d", suffix)
	newProject := func(n int) *Project {
		return &Project{
			ID:                fmt.Sprintf("slug-test-%d-%d", suffix, n),
			UserID:            userID,
			ExternalProjectID: fmt.Sprintf("slug-test-ext-%d-%d", suffix, n),
			TenantID:          userID,
			Slug:              slug,
			Name:              "Slug Test",
			Status:            "active",
			Visibility:        "public",
			WebsiteDomains:    []string{},
		}
	}

	first := newProject(1)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if err := repo.Create(ctx, newProject(2)); err == nil {
		t.Fatal("Create(duplicate active slug) succeeded, want unique violation")
	}
	if err := repo.Delete(ctx, first.ID, userID); err != nil {
		t.Fatalf("Delete(first) error = %v", err)
	}
	if err := repo.Create(ctx, newProject(3)); err != nil {
		t.Fatalf("Create(reused slug after delete) error = %v", err)
	}
	got, err := repo.GetBySlug(ctx, slug)
	if err != nil || got == nil || got.ID != newProject(3).ID {
		t.Fatalf("GetBySlug() = %+v, %v; want project 3", got, err)
	}
}
