package metrics

import (
	"testing"
	"time"

	"github.com/svgstat/svgstat/internal/analytics"
)

func TestQualityTotals(t *testing.T) {
	totals := qualityTotals(4, 1, 10, 180)
	if totals.BounceRate != 25 || totals.AveragePages != 2.5 || totals.AverageDurationSeconds != 45 {
		t.Fatalf("totals = %#v", totals)
	}
	if empty := qualityTotals(0, 0, 0, 0); empty.BounceRate != 0 || empty.AveragePages != 0 || empty.AverageDurationSeconds != 0 {
		t.Fatalf("empty totals = %#v", empty)
	}
}

func TestBuildSessionQualityReport(t *testing.T) {
	stats := emptySessionQualityStats()
	stats.Sessions = 5
	stats.Bounces = 2
	stats.SessionPageviews = 12
	stats.SessionDurationSeconds = 300
	stats.Entrances = map[string]int64{"/": 2, "/pricing": 3}
	stats.Exits = map[string]int64{"/checkout": 2, "/pricing": 3}
	stats.PageFlows = map[string]int64{"/pricing\x1f/checkout": 4, "/\x1f/pricing": 2}
	stats.SessionSegments = map[string]map[string]analytics.SessionQualityCounts{
		"source": {
			"newsletter": {Sessions: 3, Bounces: 1, Pageviews: 9, DurationSeconds: 240},
			"direct":     {Sessions: 2, Bounces: 1, Pageviews: 3, DurationSeconds: 60},
		},
	}
	report := buildSessionQualityReport("project-1", 7, time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), stats)
	if report.Totals.BounceRate != 40 || report.Totals.AveragePages != 2.4 || report.Totals.AverageDurationSeconds != 60 {
		t.Fatalf("totals = %#v", report.Totals)
	}
	if report.Entrances[0].Path != "/pricing" || report.Flows[0].From != "/pricing" || report.Flows[0].To != "/checkout" {
		t.Fatalf("pages=%#v flows=%#v", report.Entrances, report.Flows)
	}
	segments := report.Segments["source"]
	if len(segments) != 2 || segments[0].Value != "newsletter" || segments[0].AveragePages != 3 || segments[0].AverageDurationSeconds != 80 {
		t.Fatalf("segments = %#v", segments)
	}
}

func TestAddAndMaxSessionQualityStats(t *testing.T) {
	left := emptySessionQualityStats()
	left.Sessions = 3
	left.Bounces = 2
	left.SessionPageviews = 5
	left.Entrances["/"] = 3
	left.SessionSegments["device"] = map[string]analytics.SessionQualityCounts{"mobile": {Sessions: 3, Bounces: 2}}
	right := emptySessionQualityStats()
	right.Sessions = 4
	right.Bounces = 1
	right.SessionPageviews = 7
	right.Entrances["/"] = 2
	right.Entrances["/pricing"] = 2
	right.SessionSegments["device"] = map[string]analytics.SessionQualityCounts{"mobile": {Sessions: 4, Bounces: 1}}

	merged := latestSessionQualityStats(left, right)
	if merged.Sessions != 4 || merged.Bounces != 1 || merged.Entrances["/"] != 2 || merged.Entrances["/pricing"] != 2 || merged.SessionSegments["device"]["mobile"].Sessions != 4 {
		t.Fatalf("merged = %#v", merged)
	}
	if fallback := latestSessionQualityStats(right, left); fallback != right {
		t.Fatalf("lower live snapshot should not replace stored snapshot: %#v", fallback)
	}
	older := emptySessionQualityStats()
	older.Sessions, older.SessionPageviews = 4, 6
	if fallback := latestSessionQualityStats(right, older); fallback != right {
		t.Fatalf("lower pageview snapshot should not replace stored snapshot: %#v", fallback)
	}

	total := emptySessionQualityStats()
	addSessionQualityStats(total, left)
	addSessionQualityStats(total, right)
	if total.Sessions != 7 || total.Bounces != 3 || total.Entrances["/"] != 5 || total.SessionSegments["device"]["mobile"].Sessions != 7 {
		t.Fatalf("total = %#v", total)
	}
}

func TestGetSessionQualityRejectsUnsupportedRange(t *testing.T) {
	service := New(nil, nil)
	if _, err := service.GetSessionQuality(t.Context(), "project-1", 14, time.Now()); err == nil {
		t.Fatal("unsupported range was accepted")
	}
}
