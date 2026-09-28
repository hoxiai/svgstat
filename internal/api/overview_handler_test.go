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
	"github.com/hoxiai/svgstat/internal/metrics"
	"github.com/hoxiai/svgstat/internal/project"
)

type overviewLiveStatsReader struct {
	pv int64
	uv int64
	ip int64
}

func (m *overviewLiveStatsReader) GetTodayStats(ctx context.Context, projectID string) (*analytics.DailyStats, error) {
	return &analytics.DailyStats{
		ProjectID: projectID,
		Date:      time.Now().UTC().Format("2006-01-02"),
		PV:        m.pv,
		UV:        m.uv,
		IP:        m.ip,
		Hourly: map[string]map[string]int64{
			"pv": {"00": m.pv},
			"uv": {"00": m.uv},
			"ip": {"00": m.ip},
		},
	}, nil
}

func (m *overviewLiveStatsReader) GetInstallationTimes(ctx context.Context, projectID string) (*analytics.InstallationTimes, error) {
	return nil, nil
}

func (m *overviewLiveStatsReader) GetRealtimeStats(ctx context.Context, projectID string, now time.Time) (*analytics.RealtimeStats, error) {
	return nil, nil
}

func TestTodayOverview_API_Endpoint(t *testing.T) {
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		UserID: "user-1",
		Slug:   "test-overview-project",
	}

	live := &overviewLiveStatsReader{pv: 42, uv: 10, ip: 8}
	metricsSvc := metrics.New(nil, live)
	repo := &streamTestProjectRepo{project: proj}
	app := &App{
		metrics:     metricsSvc,
		projectRepo: repo,
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/v1/projects/{id}/stats/overview", app.handleGetProjectTodayOverview).Methods("GET")

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/projects/%s/stats/overview", projectID), nil)
	user := &auth.User{ID: "user-1", Email: "owner@example.com"}
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool                  `json:"success"`
		Data    metrics.TodayOverview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if resp.Data.ProjectID != projectID {
		t.Errorf("expected projectID %s, got %s", projectID, resp.Data.ProjectID)
	}
	if resp.Data.PV != 42 || resp.Data.UV != 10 || resp.Data.IP != 8 {
		t.Errorf("expected PV=42, UV=10, IP=8, got PV=%d, UV=%d, IP=%d", resp.Data.PV, resp.Data.UV, resp.Data.IP)
	}
	if resp.Data.AvgPageviewsPerUser != 4.2 {
		t.Errorf("expected AvgPageviewsPerUser=4.2, got %v", resp.Data.AvgPageviewsPerUser)
	}
	if len(resp.Data.YesterdayHourly) != 24 {
		t.Errorf("expected 24 yesterday hourly buckets, got %d", len(resp.Data.YesterdayHourly))
	}
}

func TestTodayOverview_API_NotFoundForNonOwner(t *testing.T) {
	projectID := fmt.Sprintf("test-proj-%d", time.Now().UnixNano())
	proj := &project.Project{
		ID:     projectID,
		UserID: "user-1",
		Slug:   "test-overview-project",
	}

	repo := &streamTestProjectRepo{project: proj}
	app := &App{
		projectRepo: repo,
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/projects/%s/stats/overview", projectID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": projectID})
	user := &auth.User{ID: "other-user", Email: "other@example.com"}
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	app.handleGetProjectTodayOverview(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for non-owner, got %d", rec.Code)
	}
}
