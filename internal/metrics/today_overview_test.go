package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/analytics"
)

func TestTodayOverview_IncludesAIVisits(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 35, 0, 0, time.UTC)
	projectID := "test-proj-ai"

	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-28",
			PV:        150,
			UV:        60,
			IP:        50,
			AIVisits:  12,
		},
	}

	service := newForTest(nil, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.TodayAI != 12 {
		t.Errorf("TodayAI = %d, want 12", overview.TodayAI)
	}
}

func TestTodayOverview_ZeroAIVisitsWhenNoneRecorded(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 35, 0, 0, time.UTC)
	projectID := "test-proj-no-ai"

	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-28",
			PV:        100,
			UV:        40,
			IP:        30,
			AIVisits:  0,
		},
	}

	service := newForTest(nil, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.TodayAI != 0 {
		t.Errorf("TodayAI = %d, want 0", overview.TodayAI)
	}
}
