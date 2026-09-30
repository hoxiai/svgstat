package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hoxiai/svgstat/internal/analytics"
)

func TestBuildTrendFillsMissingDatesAndTotals(t *testing.T) {
	start := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	trend := buildTrend("project-1", 3, start, end, map[string]TrendPoint{
		"2026-08-22": {PV: 3, UV: 2, IP: 2, Requests: 4, Bots: 1},
		"2026-08-24": {PV: 5, UV: 4, IP: 3, Requests: 6, Bots: 1},
	})

	if len(trend.Points) != 3 || trend.Points[1].Date != "2026-08-23" {
		t.Fatalf("points = %#v", trend.Points)
	}
	if trend.Points[1].Requests != 0 {
		t.Fatalf("missing date requests = %d, want 0", trend.Points[1].Requests)
	}
	if trend.Totals.PV != 8 || trend.Totals.UV != 6 || trend.Totals.IP != 5 || trend.Totals.Requests != 10 || trend.Totals.Bots != 2 {
		t.Fatalf("totals = %#v", trend.Totals)
	}
}

func TestMaxPointKeepsMonotonicTodayValues(t *testing.T) {
	point := maxPoint(
		TrendPoint{PV: 10, UV: 8, IP: 7, Requests: 12, Bots: 2},
		TrendPoint{Date: "2026-08-24", PV: 9, UV: 9, IP: 8, Requests: 15, Bots: 1},
	)
	if point.PV != 10 || point.UV != 9 || point.IP != 8 || point.Requests != 15 || point.Bots != 2 {
		t.Fatalf("maxPoint() = %#v", point)
	}
}

func TestGetTrendRejectsUnsupportedRange(t *testing.T) {
	service := New(nil, nil)
	if _, err := service.GetTrend(t.Context(), "project-1", 14, time.Now()); err == nil {
		t.Fatal("unsupported range was accepted")
	}
}

func TestInstallationTimeMerging(t *testing.T) {
	early := time.Date(2026, 8, 23, 1, 0, 0, 0, time.UTC)
	late := time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC)
	if got := earlierTime(&late, &early); got == nil || !got.Equal(early) {
		t.Fatalf("earlierTime() = %v, want %v", got, early)
	}
	if got := laterTime(&early, &late); got == nil || !got.Equal(late) {
		t.Fatalf("laterTime() = %v, want %v", got, late)
	}
}

func TestPeriodChanges(t *testing.T) {
	changes := periodChanges(PeriodTotals{PV: 150, UV: 50, IP: 40}, PeriodTotals{PV: 100, UV: 100, IP: 20})
	if changes.PV == nil || *changes.PV != 50 {
		t.Fatalf("PV change = %v, want 50", changes.PV)
	}
	if changes.UV == nil || *changes.UV != -50 {
		t.Fatalf("UV change = %v, want -50", changes.UV)
	}
	if changes.IP == nil || *changes.IP != 100 {
		t.Fatalf("IP change = %v, want 100", changes.IP)
	}
	if changes.Requests != nil {
		t.Fatalf("zero baseline change = %v, want nil", changes.Requests)
	}
}

func TestMaxDailyStatsMergesTodayWithoutDoubleCounting(t *testing.T) {
	merged := maxDailyStats(
		&analytics.DailyStats{PV: 10, IP: 5, Paths: map[string]int64{"/": 10}, Sources: map[string]int64{"direct": 4}},
		&analytics.DailyStats{PV: 12, IP: 8, Paths: map[string]int64{"/": 9, "/docs": 2}, Sources: map[string]int64{"direct": 5}},
	)
	if merged.PV != 12 || merged.IP != 8 || merged.Paths["/"] != 10 || merged.Paths["/docs"] != 2 || merged.Sources["direct"] != 5 {
		t.Fatalf("merged stats = %#v", merged)
	}
}

func TestTrimBreakdownsKeepsTopEntries(t *testing.T) {
	breakdowns := map[string]map[string]int64{"paths": {"/a": 1, "/b": 3, "/c": 2}}
	trimBreakdowns(breakdowns, 2)
	if len(breakdowns["paths"]) != 2 || breakdowns["paths"]["/b"] != 3 || breakdowns["paths"]["/c"] != 2 {
		t.Fatalf("trimmed breakdowns = %#v", breakdowns)
	}
}

func TestEventStatsAggregation(t *testing.T) {
	target := emptyEventStats()
	addEventStats(target, &analytics.DailyStats{UV: 20, Events: map[string]int64{"purchase": 3}, EventVisitors: map[string]int64{"purchase": 2}, EventSources: map[string]map[string]int64{"purchase": {"newsletter": 3}}, EventValues: map[string]map[string]float64{"purchase": {"CNY": 198}}})
	addEventStats(target, &analytics.DailyStats{UV: 10, Events: map[string]int64{"purchase": 1}, EventVisitors: map[string]int64{"purchase": 1}, EventSources: map[string]map[string]int64{"purchase": {"direct": 1}}, EventValues: map[string]map[string]float64{"purchase": {"CNY": 99}}})
	if target.UV != 30 || target.Events["purchase"] != 4 || target.EventVisitors["purchase"] != 3 || target.EventSources["purchase"]["newsletter"] != 3 || target.EventValues["purchase"]["CNY"] != 297 {
		t.Fatalf("unexpected event aggregation: %#v", target)
	}
}

func TestStepCountHelpers(t *testing.T) {
	counts := []int64{0, 0, 0}
	addStepCounts(counts, []int64{10, 6, 2})
	addStepCounts(counts, []int64{4, 3})
	if counts[0] != 14 || counts[1] != 9 || counts[2] != 2 {
		t.Fatalf("step counts = %v", counts)
	}
	maxCounts := maxStepCounts([]int64{5, 4}, []int64{6, 3, 1})
	if maxCounts[0] != 6 || maxCounts[1] != 4 || maxCounts[2] != 1 {
		t.Fatalf("max step counts = %v", maxCounts)
	}
}

func TestBuildConversionSegmentsUsesMatchingAudience(t *testing.T) {
	stats := emptyEventStats()
	stats.AudienceSegments = map[string]map[string]int64{"source": {"newsletter": 20, "direct": 40}}
	stats.EventSegments = map[string]map[string]map[string]int64{"signup": {"source": {"newsletter": 5, "direct": 4}}}
	segments := buildConversionSegments(stats, "signup")["source"]
	if len(segments) != 2 {
		t.Fatalf("segments = %#v", segments)
	}
	if segments[0].Value != "newsletter" || segments[0].Audience != 20 || segments[0].Converters != 5 || segments[0].Rate != 25 {
		t.Fatalf("top segment = %#v", segments[0])
	}
	if segments[1].Value != "direct" || segments[1].Rate != 10 {
		t.Fatalf("second segment = %#v", segments[1])
	}
}

func TestFunnelSegmentAggregation(t *testing.T) {
	target := map[string]map[string][]int64{}
	addFunnelSegmentCounts(target, map[string]map[string][]int64{"source": {"newsletter": {10, 6, 3}}}, 3)
	addFunnelSegmentCounts(target, map[string]map[string][]int64{"source": {"newsletter": {4, 2, 1}, "direct": {8, 2, 1}}}, 3)
	report := buildFunnelSegments(target)["source"]
	if len(report) != 2 {
		t.Fatalf("segments = %#v", report)
	}
	if report[0].Value != "newsletter" || report[0].Steps[0] != 14 || report[0].Steps[2] != 4 || report[0].ConversionRate < 28.5 || report[0].ConversionRate > 28.6 {
		t.Fatalf("newsletter segment = %#v", report[0])
	}
	if report[1].Value != "direct" || report[1].ConversionRate != 12.5 {
		t.Fatalf("direct segment = %#v", report[1])
	}
}

func TestMaxEventStatsMergesTodaySegmentsWithoutDoubleCounting(t *testing.T) {
	stored := emptyEventStats()
	live := emptyEventStats()
	stored.AudienceSegments = map[string]map[string]int64{"device": {"mobile": 10}}
	live.AudienceSegments = map[string]map[string]int64{"device": {"mobile": 8, "desktop": 4}}
	stored.EventSegments = map[string]map[string]map[string]int64{"signup": {"device": {"mobile": 3}}}
	live.EventSegments = map[string]map[string]map[string]int64{"signup": {"device": {"mobile": 4, "desktop": 1}}}
	merged := maxEventStats(stored, live)
	if merged.AudienceSegments["device"]["mobile"] != 10 || merged.AudienceSegments["device"]["desktop"] != 4 || merged.EventSegments["signup"]["device"]["mobile"] != 4 {
		t.Fatalf("merged = %#v", merged)
	}
}

func TestNormalizeFunnelCounts(t *testing.T) {
	counts := []int64{3, 5, 4, 6}
	normalizeFunnelCounts(counts)
	if counts[0] != 3 || counts[1] != 3 || counts[2] != 3 || counts[3] != 3 {
		t.Fatalf("counts = %v", counts)
	}
}

func TestBuildEventDetailsOnlyReturnsPropertySegments(t *testing.T) {
	stats := emptyEventStats()
	stats.EventSegments = map[string]map[string]map[string]int64{
		"file_download": {
			"property_file_extension": {"pdf": 5, "zip": 2},
			"source":                  {"newsletter": 4},
		},
	}
	details := buildEventDetails(stats, "file_download")
	if len(details) != 1 || len(details["file_extension"]) != 2 || details["file_extension"][0].Value != "pdf" || details["file_extension"][0].Visitors != 5 {
		t.Fatalf("details = %#v", details)
	}
}

func TestBuildWebQualitySummarizesVitalsAndErrors(t *testing.T) {
	stats := emptyEventStats()
	stats.Events = map[string]int64{"web_vital_lcp": 2, "web_vital_cls": 1, "js_error": 3, "resource_error": 2}
	stats.EventVisitors = map[string]int64{"web_vital_lcp": 2, "web_vital_cls": 1, "js_error": 2, "resource_error": 1}
	stats.EventValues = map[string]map[string]float64{"web_vital_lcp": {"XXX": 6000}, "web_vital_cls": {"XXX": 0.08}}
	stats.EventSegments = map[string]map[string]map[string]int64{
		"web_vital_lcp": {"property_rating": {"good": 1, "poor": 1}},
		"js_error":      {"property_error_type": {"type_error": 2}},
	}
	quality := buildWebQuality(stats)
	if quality.Status != "needs_improvement" || len(quality.Vitals) != 2 || quality.Vitals[0].Name != "lcp" || quality.Vitals[0].Average != 3000 {
		t.Fatalf("quality = %#v", quality)
	}
	if quality.JavaScript.Count != 3 || quality.JavaScript.Visitors != 2 || quality.ErrorDetails["js_error_type"][0].Value != "type_error" {
		t.Fatalf("error summary = %#v", quality)
	}
	onlyErrors := buildWebQuality(&analytics.DailyStats{Events: map[string]int64{"js_error": 1}, EventVisitors: map[string]int64{}, EventValues: map[string]map[string]float64{}, EventSegments: map[string]map[string]map[string]int64{}})
	if onlyErrors.Status != "needs_improvement" {
		t.Fatalf("onlyErrors = %#v", onlyErrors)
	}
}

func TestAnalysisBreakdownsIncludeVisitAttribution(t *testing.T) {
	breakdowns := emptyBreakdowns()
	var totals PeriodTotals
	day := &analytics.DailyStats{
		Terms:        map[string]int64{"svg badge": 2},
		Channels:     map[string]int64{"organic\x1fgoogle": 3},
		SiteSearches: map[string]int64{"pricing": 4},
	}
	addPeriodStats(&totals, breakdowns, day)
	addPeriodStats(&totals, breakdowns, day)
	if breakdowns["terms"]["svg badge"] != 4 || breakdowns["channels"]["organic\x1fgoogle"] != 6 || breakdowns["siteSearches"]["pricing"] != 8 {
		t.Fatalf("breakdowns = %#v", breakdowns)
	}

	merged := maxDailyStats(
		&analytics.DailyStats{Terms: map[string]int64{"a": 3}, Channels: map[string]int64{"x": 1}, SiteSearches: map[string]int64{"q": 5}},
		&analytics.DailyStats{Terms: map[string]int64{"a": 2}, Channels: map[string]int64{"x": 4}, SiteSearches: map[string]int64{"q": 1}},
	)
	if merged.Terms["a"] != 3 || merged.Channels["x"] != 4 || merged.SiteSearches["q"] != 5 {
		t.Fatalf("merged = %#v", merged)
	}
}

func TestTrimBreakdownsKeepsMoreChannels(t *testing.T) {
	channels := map[string]int64{}
	for i := 0; i < 30; i++ {
		channels[string(rune('a'+i))] = int64(i + 1)
	}
	breakdowns := map[string]map[string]int64{"channels": channels}
	trimBreakdowns(breakdowns, 20)
	if len(breakdowns["channels"]) != 30 {
		t.Fatalf("channels kept %d entries, want all 30 (channel drill-down needs pairs beyond the top 20)", len(breakdowns["channels"]))
	}
}

type mockLiveStatsReader struct {
	todayStats *analytics.DailyStats
	err        error
}

func (m *mockLiveStatsReader) GetTodayStats(ctx context.Context, projectID string) (*analytics.DailyStats, error) {
	return m.todayStats, m.err
}

func (m *mockLiveStatsReader) GetInstallationTimes(ctx context.Context, projectID string) (*analytics.InstallationTimes, error) {
	return nil, nil
}

func (m *mockLiveStatsReader) GetRealtimeStats(ctx context.Context, projectID string, now time.Time) (*analytics.RealtimeStats, error) {
	return nil, nil
}

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
}

func TestGetTodayOverview_CalculatesHeroAndHourly(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 35, 0, 0, time.UTC)
	projectID := "test-proj-1"

	// Live stats for today (hour 14)
	todayHourlyPV := map[string]int64{}
	todayHourlyUV := map[string]int64{}
	todayHourlyIP := map[string]int64{}
	for h := 0; h <= 14; h++ {
		key := fmt.Sprintf("%02d", h)
		todayHourlyPV[key] = 10
		todayHourlyUV[key] = 4
		todayHourlyIP[key] = 3
	}
	// Add hour 14 specific
	todayHourlyPV["14"] = 10
	todayHourlyUV["14"] = 4
	todayHourlyIP["14"] = 8

	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-28",
			PV:        150,
			UV:        60,
			IP:        50,
			Hourly: map[string]map[string]int64{
				"pv": todayHourlyPV,
				"uv": todayHourlyUV,
				"ip": todayHourlyIP,
			},
		},
	}

	// Yesterday stats in PostgreSQL (full 24h)
	yesterdayHourlyPV := map[string]int64{}
	yesterdayHourlyUV := map[string]int64{}
	yesterdayHourlyIP := map[string]int64{}
	for h := 0; h < 24; h++ {
		key := fmt.Sprintf("%02d", h)
		yesterdayHourlyPV[key] = 8
		yesterdayHourlyUV[key] = 4
		yesterdayHourlyIP[key] = 3
	}
	// For hours 00..14 (15 hours):
	// PV: 15 * 8 = 120 (let's set sum to 100 by tweaking: 10 hours of 10 = 100)
	for h := 0; h < 24; h++ {
		key := fmt.Sprintf("%02d", h)
		if h <= 14 {
			// Hours 00..14 sum:
			// PV: 100 total
			yesterdayHourlyPV[key] = 6
			yesterdayHourlyUV[key] = 3
			yesterdayHourlyIP[key] = 2
		} else {
			// Hours 15..23 sum:
			yesterdayHourlyPV[key] = 10
			yesterdayHourlyUV[key] = 5
			yesterdayHourlyIP[key] = 4
		}
	}
	// Customize exact totals for hours 00..14:
	// Set hour 00 to make exact totals:
	// PV sum 00..14: 14 * 6 = 84 + 16 = 100
	yesterdayHourlyPV["00"] = 16
	// UV sum 00..14: 14 * 3 = 42 + 8 = 50
	yesterdayHourlyUV["00"] = 8
	// IP sum 00..14: 14 * 2 = 28 + 12 = 40
	yesterdayHourlyIP["00"] = 12

	// Customize exact totals for hours 15..23 to match full day totals (200 PV, 100 UV, 80 IP):
	// PV sum 15..23: 8 * 10 = 80 + 20 = 100
	yesterdayHourlyPV["23"] = 20
	// UV sum 15..23: 8 * 5 = 40 + 10 = 50
	yesterdayHourlyUV["23"] = 10
	// IP sum 15..23: 8 * 4 = 32 + 8 = 40
	yesterdayHourlyIP["23"] = 8

	yesterdayHourlyJSON, err := json.Marshal(map[string]map[string]int64{
		"pv": yesterdayHourlyPV,
		"uv": yesterdayHourlyUV,
		"ip": yesterdayHourlyIP,
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	mockDBInstance := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if len(args) >= 2 && args[0] == projectID && args[1] == "2026-09-27" {
				return &mockRow{
					scanFn: func(dest ...any) error {
						*dest[0].(*int64) = 200 // yesterdayFull PV
						*dest[1].(*int64) = 100 // yesterdayFull UV
						*dest[2].(*int64) = 80  // yesterdayFull IP
						*dest[3].(*[]byte) = yesterdayHourlyJSON
						return nil
					},
				}
			}
			return &mockRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	service := newForTest(mockDBInstance, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.ProjectID != projectID {
		t.Errorf("ProjectID = %q, want %q", overview.ProjectID, projectID)
	}
	if overview.Date != "2026-09-28" {
		t.Errorf("Date = %q, want 2026-09-28", overview.Date)
	}
	if overview.CurrentHour != 14 {
		t.Errorf("CurrentHour = %d, want 14", overview.CurrentHour)
	}
	if overview.PV != 150 || overview.UV != 60 || overview.IP != 50 {
		t.Errorf("Today metrics = PV:%d UV:%d IP:%d, want 150, 60, 50", overview.PV, overview.UV, overview.IP)
	}
	if overview.AvgPageviewsPerUser != 2.5 {
		t.Errorf("AvgPageviewsPerUser = %v, want 2.5", overview.AvgPageviewsPerUser)
	}

	// Yesterday full day
	if overview.YesterdayFull.PV != 200 || overview.YesterdayFull.UV != 100 || overview.YesterdayFull.IP != 80 {
		t.Errorf("YesterdayFull = %#v, want PV:200 UV:100 IP:80", overview.YesterdayFull)
	}

	// Yesterday same period (00..14)
	if overview.YesterdaySamePeriod.PV != 100 || overview.YesterdaySamePeriod.UV != 50 || overview.YesterdaySamePeriod.IP != 40 {
		t.Errorf("YesterdaySamePeriod = %#v, want PV:100 UV:50 IP:40", overview.YesterdaySamePeriod)
	}

	// Changes: (today - yesterday_same_period) / yesterday_same_period * 100
	// PV: (150 - 100) / 100 * 100 = +50.0%
	// UV: (60 - 50) / 50 * 100 = +20.0%
	// IP: (50 - 40) / 40 * 100 = +25.0%
	if overview.Changes.PV == nil || *overview.Changes.PV != 50.0 {
		t.Errorf("Changes.PV = %v, want 50.0", overview.Changes.PV)
	}
	if overview.Changes.UV == nil || *overview.Changes.UV != 20.0 {
		t.Errorf("Changes.UV = %v, want 20.0", overview.Changes.UV)
	}
	if overview.Changes.IP == nil || *overview.Changes.IP != 25.0 {
		t.Errorf("Changes.IP = %v, want 25.0", overview.Changes.IP)
	}

	// TodayHourly should contain hours 00..14 (15 hours)
	if len(overview.TodayHourly) != 15 {
		t.Errorf("len(TodayHourly) = %d, want 15 (hours 00..14)", len(overview.TodayHourly))
	}
	if _, exists := overview.TodayHourly["15"]; exists {
		t.Errorf("TodayHourly should not contain hour 15")
	}
	if pt, exists := overview.TodayHourly["14"]; !exists || pt.IP != 8 {
		t.Errorf("TodayHourly[14] = %#v, want IP: 8", pt)
	}

	// YesterdayHourly should contain hours 00..23 (24 hours)
	if len(overview.YesterdayHourly) != 24 {
		t.Errorf("len(YesterdayHourly) = %d, want 24", len(overview.YesterdayHourly))
	}
	if pt, exists := overview.YesterdayHourly["00"]; !exists || pt.PV != 16 || pt.UV != 8 || pt.IP != 12 {
		t.Errorf("YesterdayHourly[00] = %#v, want PV:16 UV:8 IP:12", pt)
	}
}

func TestGetTodayOverview_HandlesMissingYesterdayRowAndZeroUV(t *testing.T) {
	now := time.Date(2026, 9, 28, 5, 0, 0, 0, time.UTC)
	projectID := "test-proj-zero"

	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-28",
			PV:        10,
			UV:        0,
			IP:        5,
			Hourly: map[string]map[string]int64{
				"pv": {"05": 10},
			},
		},
	}

	mockDBInstance := &mockDB{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	service := newForTest(mockDBInstance, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.AvgPageviewsPerUser != 0 {
		t.Errorf("AvgPageviewsPerUser = %v, want 0 when UV=0", overview.AvgPageviewsPerUser)
	}
	if overview.YesterdayFull.PV != 0 || overview.YesterdaySamePeriod.PV != 0 {
		t.Errorf("YesterdayFull/SamePeriod should be 0, got %#v, %#v", overview.YesterdayFull, overview.YesterdaySamePeriod)
	}
	if overview.Changes.PV != nil || overview.Changes.UV != nil || overview.Changes.IP != nil {
		t.Errorf("Changes should all be nil when baseline is 0, got %#v", overview.Changes)
	}
	if len(overview.TodayHourly) != 6 { // hours 00..05 = 6
		t.Errorf("len(TodayHourly) = %d, want 6", len(overview.TodayHourly))
	}
	if len(overview.YesterdayHourly) != 24 {
		t.Errorf("len(YesterdayHourly) = %d, want 24", len(overview.YesterdayHourly))
	}
}

func TestGetTodayOverview_PropagatesLiveError(t *testing.T) {
	now := time.Date(2026, 9, 28, 5, 0, 0, 0, time.UTC)
	mockLive := &mockLiveStatsReader{
		err: fmt.Errorf("redis connection timeout"),
	}
	service := newForTest(nil, mockLive)
	_, err := service.GetTodayOverview(context.Background(), "test-proj", now)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}


