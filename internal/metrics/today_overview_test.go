package metrics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

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

func TestTodayOverview_YesterdaySamePeriodUVAndIPScale(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) // noon
	projectID := "test-proj-scale"

	// Mock live stats: today has 50 unique visitors, 40 IPs so far
	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-29",
			PV:        200,
			UV:        50,
			IP:        40,
			Hourly: map[string]map[string]int64{
				"pv": {"10": 100, "11": 100},
				"uv": {"10": 25, "11": 25},
				"ip": {"10": 20, "11": 20},
			},
		},
	}

	// Mock DB with yesterday's stats:
	// yesterdayFull: PV=400, UV=100, IP=80
	// yesterday hourly UV sums to 200 (due to repeat visits across hours)
	// up to hour 12, yesterday hourly UV sum is 100 (50% of the day's activity)
	hourlyData := map[string]map[string]int64{
		"pv": {"10": 100, "11": 100, "14": 100, "15": 100},
		"uv": {"10": 50, "11": 50, "14": 50, "15": 50},
		"ip": {"10": 40, "11": 40, "14": 40, "15": 40},
	}
	hourlyJSON, _ := json.Marshal(hourlyData)

	mockPool := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 400
					*dest[1].(*int64) = 100
					*dest[2].(*int64) = 80
					*dest[3].(*[]byte) = hourlyJSON
					return nil
				},
			}
		},
	}

	service := newForTest(mockPool, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// YesterdaySamePeriod UV should be ~50% of yesterdayFull (100 * 100 / 200 = 50), NOT the raw hourly sum of 100
	if overview.YesterdaySamePeriod.UV != 50 {
		t.Errorf("expected YesterdaySamePeriod.UV = 50, got %d", overview.YesterdaySamePeriod.UV)
	}
	if overview.YesterdaySamePeriod.IP != 40 {
		t.Errorf("expected YesterdaySamePeriod.IP = 40, got %d", overview.YesterdaySamePeriod.IP)
	}
	// Changes for UV: today (50) vs yesterdaySamePeriod (50) should be 0%, not -50%
	if overview.Changes.UV == nil || *overview.Changes.UV != 0.0 {
		t.Errorf("expected Changes.UV = 0.0, got %v", overview.Changes.UV)
	}
}

func TestTodayOverview_YesterdaySamePeriodAtEndOfDay(t *testing.T) {
	now := time.Date(2026, 9, 29, 23, 15, 0, 0, time.UTC) // 23:15
	projectID := "test-proj-eod"

	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-29",
			PV:        300,
			UV:        80,
			IP:        60,
		},
	}

	hourlyData := map[string]map[string]int64{
		"pv": {"10": 100, "11": 100},
		"uv": {"10": 60, "11": 60}, // sum = 120, but yesterdayFull.UV = 100
		"ip": {"10": 50, "11": 50}, // sum = 100, but yesterdayFull.IP = 80
	}
	hourlyJSON, _ := json.Marshal(hourlyData)

	mockPool := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 400
					*dest[1].(*int64) = 100
					*dest[2].(*int64) = 80
					*dest[3].(*[]byte) = hourlyJSON
					return nil
				},
			}
		},
	}

	service := newForTest(mockPool, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.YesterdaySamePeriod.UV != 100 {
		t.Errorf("expected YesterdaySamePeriod.UV = 100, got %d", overview.YesterdaySamePeriod.UV)
	}
	if overview.YesterdaySamePeriod.IP != 80 {
		t.Errorf("expected YesterdaySamePeriod.IP = 80, got %d", overview.YesterdaySamePeriod.IP)
	}
}

