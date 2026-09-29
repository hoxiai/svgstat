package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/hoxiai/svgstat/internal/analytics"
	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/hoxiai/svgstat/internal/counter"
	"github.com/hoxiai/svgstat/internal/project"
	"github.com/hoxiai/svgstat/internal/renderer"
	"github.com/hoxiai/svgstat/internal/requestmeta"
	"github.com/joho/godotenv"
)

type svgTestProjectRepo struct {
	project.Repository
	project *project.Project
}

func (r *svgTestProjectRepo) GetBySlug(ctx context.Context, slug string) (*project.Project, error) {
	if r.project != nil && r.project.Slug == slug {
		return r.project, nil
	}
	return nil, nil
}

func (r *svgTestProjectRepo) GetCounter(ctx context.Context, projectID, counterName string) (int64, error) {
	return 0, nil
}

func newSVGTestAPIApp(t *testing.T, proj *project.Project) (*App, *analytics.Analytics, string) {
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
		if proj != nil && proj.Slug != "" {
			_ = c.Delete(context.Background(), "runtime:project:"+proj.Slug)
		}
		c.Close()
	})

	repo := &svgTestProjectRepo{project: proj}
	analyticsSvc := analytics.New(c, repo, nil, time.Hour, "salt")
	runtimeCache := project.NewRuntimeCache(repo, c, time.Hour)
	counterSvc := counter.New(c, repo, time.Hour)
	rendererSvc := renderer.New()
	reqResolver := requestmeta.NewResolver(nil)

	app := &App{
		analytics:   analyticsSvc,
		projectRepo: repo,
		runtime:     runtimeCache,
		counter:     counterSvc,
		renderer:    rendererSvc,
		requestMeta: reqResolver,
	}
	return app, analyticsSvc, projectID
}

func TestSVGTracking_RecordsPageviewAndStream(t *testing.T) {
	nowNano := time.Now().UnixNano()
	projectID := fmt.Sprintf("test-svg-%d", nowNano)
	slug := fmt.Sprintf("my-svg-proj-%d", nowNano)
	proj := &project.Project{
		ID:            projectID,
		UserID:        "user-1",
		Slug:          slug,
		Status:        "active",
		RenderEnabled: true,
		BadgeEnabled:  true,
	}

	app, analyticsSvc, _ := newSVGTestAPIApp(t, proj)
	ctx := context.Background()

	// 1. Counter request: non-preview with Referer path
	req := httptest.NewRequest("GET", fmt.Sprintf("/svg/%s/counter/views.svg", slug), nil)
	req = mux.SetURLVars(req, map[string]string{
		"projectSlug": proj.Slug,
		"name":        "views",
	})
	req.Header.Set("Referer", "https://myblog.com/posts/article-1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.RemoteAddr = "114.240.10.1:1234"

	rec := httptest.NewRecorder()
	app.handleCounterSVG(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	stats, err := analyticsSvc.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}

	if stats.Requests != 1 {
		t.Errorf("expected requests = 1, got %d", stats.Requests)
	}
	if stats.PV != 1 {
		t.Errorf("expected pv = 1, got %d", stats.PV)
	}
	if stats.UV != 1 {
		t.Errorf("expected uv = 1, got %d", stats.UV)
	}
	if stats.IP != 1 {
		t.Errorf("expected ip = 1, got %d", stats.IP)
	}
	if stats.Paths["/posts/article-1"] != 1 {
		t.Errorf("expected path /posts/article-1 = 1, got %v", stats.Paths)
	}

	stream, err := analyticsSvc.GetVisitStream(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("GetVisitStream failed: %v", err)
	}
	if len(stream) != 1 {
		t.Fatalf("expected 1 stream item, got %d", len(stream))
	}
	if stream[0].Path != "/posts/article-1" {
		t.Errorf("expected stream path = /posts/article-1, got %q", stream[0].Path)
	}

	// 2. Badge request: non-preview with Referer path
	reqBadge := httptest.NewRequest("GET", fmt.Sprintf("/svg/%s/badge/stars.svg", slug), nil)
	reqBadge = mux.SetURLVars(reqBadge, map[string]string{
		"projectSlug": proj.Slug,
		"name":        "stars",
	})
	reqBadge.Header.Set("Referer", "https://myblog.com/docs/page-2")
	reqBadge.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	reqBadge.RemoteAddr = "114.240.10.2:1234"

	recBadge := httptest.NewRecorder()
	app.handleBadgeSVG(recBadge, reqBadge)

	if recBadge.Code != http.StatusOK {
		t.Fatalf("expected badge status 200, got %d: %s", recBadge.Code, recBadge.Body.String())
	}

	stats, err = analyticsSvc.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}
	if stats.Requests != 2 {
		t.Errorf("expected requests = 2, got %d", stats.Requests)
	}
	if stats.PV != 2 {
		t.Errorf("expected pv = 2, got %d", stats.PV)
	}
	if stats.Paths["/docs/page-2"] != 1 {
		t.Errorf("expected path /docs/page-2 = 1, got %v", stats.Paths)
	}

	stream, err = analyticsSvc.GetVisitStream(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("GetVisitStream failed: %v", err)
	}
	if len(stream) != 2 {
		t.Fatalf("expected 2 stream items, got %d", len(stream))
	}
	if stream[0].Path != "/docs/page-2" {
		t.Errorf("expected latest stream path = /docs/page-2, got %q", stream[0].Path)
	}

	// 3. Preview request: should NOT increment requests or pageviews
	reqPreview := httptest.NewRequest("GET", fmt.Sprintf("/svg/%s/counter/views.svg?preview=1", slug), nil)
	reqPreview = mux.SetURLVars(reqPreview, map[string]string{
		"projectSlug": proj.Slug,
		"name":        "views",
	})
	recPreview := httptest.NewRecorder()
	app.handleCounterSVG(recPreview, reqPreview)

	if recPreview.Code != http.StatusOK {
		t.Fatalf("expected preview status 200, got %d: %s", recPreview.Code, recPreview.Body.String())
	}

	stats, err = analyticsSvc.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}
	if stats.Requests != 2 {
		t.Errorf("expected requests to remain 2 after preview, got %d", stats.Requests)
	}
	if stats.PV != 2 {
		t.Errorf("expected pv to remain 2 after preview, got %d", stats.PV)
	}
}

func TestSVGTracking_ReferrerPathExtraction(t *testing.T) {
	projectID := fmt.Sprintf("test-ref-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		Slug:   "ref-project",
		Status: "active",
	}

	_, analyticsSvc, _ := newSVGTestAPIApp(t, proj)
	ctx := context.Background()

	tests := []struct {
		name     string
		referer  string
		reqPath  string
		wantPath string
	}{
		{
			name:     "referer with deep path",
			referer:  "https://myblog.com/posts/article-1",
			reqPath:  "/svg/ref-project/counter/views.svg",
			wantPath: "/posts/article-1",
		},
		{
			name:     "referer with root path",
			referer:  "https://myblog.com/",
			reqPath:  "/svg/ref-project/counter/views.svg",
			wantPath: "/",
		},
		{
			name:     "referer without path defaults to root path /",
			referer:  "https://myblog.com",
			reqPath:  "/svg/ref-project/counter/views.svg",
			wantPath: "/",
		},
		{
			name:     "empty referer falls back to req.URL.Path",
			referer:  "",
			reqPath:  "/svg/ref-project/badge/status.svg",
			wantPath: "/svg/ref-project/badge/status.svg",
		},
		{
			name:     "empty referer and empty req.URL.Path falls back to /",
			referer:  "",
			reqPath:  "",
			wantPath: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subProjID := fmt.Sprintf("%s-%d", projectID, time.Now().UnixNano())
			req := httptest.NewRequest("GET", "http://example.com"+tt.reqPath, nil)
			if tt.reqPath == "" {
				req.URL = &url.URL{}
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			req.RemoteAddr = "120.24.1.1:1234"
			req.Header.Set("User-Agent", "Mozilla/5.0")

			err := analyticsSvc.TrackBadgeOrCounter(ctx, req, subProjID)
			if err != nil {
				t.Fatalf("TrackBadgeOrCounter failed: %v", err)
			}

			stats, err := analyticsSvc.GetTodayStats(ctx, subProjID)
			if err != nil {
				t.Fatalf("GetTodayStats failed: %v", err)
			}
			if stats.PV != 1 {
				t.Errorf("expected pv = 1, got %d", stats.PV)
			}
			if stats.Requests != 1 {
				t.Errorf("expected requests = 1, got %d", stats.Requests)
			}
			if stats.Paths[tt.wantPath] != 1 {
				t.Errorf("expected path %q count = 1, got paths: %v", tt.wantPath, stats.Paths)
			}
		})
	}
}

func TestSVGTracking_CamoProxyReferrerPreserved(t *testing.T) {
	proj := &project.Project{
		ID:            "camo-test-project",
		Slug:          "camo-test-slug",
		Status:        "active",
		RenderEnabled: true,
	}
	_, analyticsSvc, projectID := newSVGTestAPIApp(t, proj)
	ctx := context.Background()

	subProjID := fmt.Sprintf("%s-camo-%d", projectID, time.Now().UnixNano())
	req := httptest.NewRequest("GET", "http://example.com/svg/camo-project/counter/views.svg?page_id=github.com/my-user/my-repo", nil)
	req.Header.Set("User-Agent", "github-camo (xyz)")
	req.RemoteAddr = "120.24.1.1:1234"

	err := analyticsSvc.TrackBadgeOrCounter(ctx, req, subProjID)
	if err != nil {
		t.Fatalf("TrackBadgeOrCounter failed: %v", err)
	}

	stats, err := analyticsSvc.GetTodayStats(ctx, subProjID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}
	if stats.PV != 1 {
		t.Errorf("expected pv = 1, got %d", stats.PV)
	}

	streamItems, err := analyticsSvc.GetVisitStream(ctx, subProjID, 1)
	if err != nil {
		t.Fatalf("GetVisitStream failed: %v", err)
	}
	if len(streamItems) != 1 {
		t.Fatalf("expected 1 stream item, got %d", len(streamItems))
	}
	if streamItems[0].SourceURL != "https://github.com/my-user/my-repo" {
		t.Errorf("expected SourceURL to be https://github.com/my-user/my-repo, got %q", streamItems[0].SourceURL)
	}
}
