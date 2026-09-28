package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/hoxiai/svgstat/internal/analytics"
	"github.com/hoxiai/svgstat/internal/auth"
	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/project"
	"github.com/joho/godotenv"
)

type streamTestProjectRepo struct {
	project.Repository
	project *project.Project
}

func (r *streamTestProjectRepo) GetByIDAndUser(ctx context.Context, id, userID string) (*project.Project, error) {
	if r.project != nil && r.project.ID == id && r.project.UserID == userID {
		return r.project, nil
	}
	return nil, nil
}

func newTestAPIApp(t *testing.T, proj *project.Project) (*App, *analytics.Analytics, string) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	c, err := cache.New(config.Load())
	if err != nil {
		t.Skip("Skipping test: Redis not reachable")
	}
	projectID := proj.ID
	t.Cleanup(func() {
		keys, _ := c.GetClient().Keys(context.Background(), cache.BuildKey("project", projectID, "*")).Result()
		if len(keys) > 0 {
			_ = c.Delete(context.Background(), keys...)
		}
		c.Close()
	})

	analyticsSvc := analytics.New(c, nil, nil, time.Hour, "salt")
	repo := &streamTestProjectRepo{project: proj}
	app := &App{
		analytics:   analyticsSvc,
		projectRepo: repo,
	}
	return app, analyticsSvc, projectID
}

func TestVisitStream_API_Endpoint(t *testing.T) {
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		UserID: "user-1",
		Slug:   "test-stream-project",
	}

	app, analyticsSvc, _ := newTestAPIApp(t, proj)
	ctx := context.Background()

	// Seed 3 visits
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("https://example.com/p-%d", i), nil)
		req.RemoteAddr = fmt.Sprintf("114.240.10.%d:8080", i)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		_ = analyticsSvc.TrackPageview(ctx, req, projectID, fmt.Sprintf("/p-%d", i), "", fmt.Sprintf("v-%d", i), "")
	}

	// Make request to handleGetProjectVisitStream
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/projects/%s/visit-stream?limit=2", projectID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": projectID})
	user := &auth.User{ID: "user-1", Email: "owner@example.com"}
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	app.handleGetProjectVisitStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool                         `json:"success"`
		Data    []analytics.VisitStreamItem `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got %v", resp.Success)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Data[0].Path != "/p-3" {
		t.Errorf("expected newest path /p-3, got %s", resp.Data[0].Path)
	}
	if resp.Data[0].MaskedIP != "114.240.*.*" {
		t.Errorf("expected masked ip 114.240.*.*, got %s", resp.Data[0].MaskedIP)
	}
}

func TestVisitStream_API_NotFoundForNonOwner(t *testing.T) {
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		UserID: "user-1",
		Slug:   "test-stream-project",
	}

	app, _, _ := newTestAPIApp(t, proj)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/projects/%s/visit-stream", projectID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": projectID})
	user := &auth.User{ID: "user-other", Email: "other@example.com"}
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	app.handleGetProjectVisitStream(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVisitStream_API_LimitClamping(t *testing.T) {
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		UserID: "user-1",
		Slug:   "test-stream-project",
	}

	app, analyticsSvc, _ := newTestAPIApp(t, proj)
	ctx := context.Background()

	for i := 1; i <= 10; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("https://example.com/p-%d", i), nil)
		req.RemoteAddr = fmt.Sprintf("114.240.10.%d:8080", i)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		_ = analyticsSvc.TrackPageview(ctx, req, projectID, fmt.Sprintf("/p-%d", i), "", fmt.Sprintf("v-%d", i), "")
	}

	// Test limit=3
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/projects/%s/visit-stream?limit=3", projectID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": projectID})
	user := &auth.User{ID: "user-1", Email: "owner@example.com"}
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	app.handleGetProjectVisitStream(rec, req)

	var resp struct {
		Success bool                         `json:"success"`
		Data    []analytics.VisitStreamItem `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 items with limit=3, got %d", len(resp.Data))
	}
}

