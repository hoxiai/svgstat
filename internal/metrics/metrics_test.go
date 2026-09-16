package metrics

import (
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/analytics"
)

func TestBuildTrendFillsMissingDatesAndTotals(t *testing.T) {
	start := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	trend := buildTrend("project-1", 3, start, end, map[string]TrendPoint{
		"2026-08-22": {PV: 3, UV: 2, Requests: 4, Bots: 1},
		"2026-08-24": {PV: 5, UV: 4, Requests: 6, Bots: 1},
	})

	if len(trend.Points) != 3 || trend.Points[1].Date != "2026-08-23" {
		t.Fatalf("points = %#v", trend.Points)
	}
	if trend.Points[1].Requests != 0 {
		t.Fatalf("missing date requests = %d, want 0", trend.Points[1].Requests)
	}
	if trend.Totals.PV != 8 || trend.Totals.UV != 6 || trend.Totals.Requests != 10 || trend.Totals.Bots != 2 {
		t.Fatalf("totals = %#v", trend.Totals)
	}
}

func TestMaxPointKeepsMonotonicTodayValues(t *testing.T) {
	point := maxPoint(
		TrendPoint{PV: 10, UV: 8, Requests: 12, Bots: 2},
		TrendPoint{Date: "2026-08-24", PV: 9, UV: 9, Requests: 15, Bots: 1},
	)
	if point.PV != 10 || point.UV != 9 || point.Requests != 15 || point.Bots != 2 {
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
	changes := periodChanges(PeriodTotals{PV: 150, UV: 50}, PeriodTotals{PV: 100, UV: 100})
	if changes.PV == nil || *changes.PV != 50 {
		t.Fatalf("PV change = %v, want 50", changes.PV)
	}
	if changes.UV == nil || *changes.UV != -50 {
		t.Fatalf("UV change = %v, want -50", changes.UV)
	}
	if changes.Requests != nil {
		t.Fatalf("zero baseline change = %v, want nil", changes.Requests)
	}
}

func TestMaxDailyStatsMergesTodayWithoutDoubleCounting(t *testing.T) {
	merged := maxDailyStats(
		&analytics.DailyStats{PV: 10, Paths: map[string]int64{"/": 10}, Sources: map[string]int64{"direct": 4}},
		&analytics.DailyStats{PV: 12, Paths: map[string]int64{"/": 9, "/docs": 2}, Sources: map[string]int64{"direct": 5}},
	)
	if merged.PV != 12 || merged.Paths["/"] != 10 || merged.Paths["/docs"] != 2 || merged.Sources["direct"] != 5 {
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
