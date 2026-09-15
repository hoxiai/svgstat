package analytics

import "testing"

func TestParseSessionSegments(t *testing.T) {
	segments := map[string]map[string]SessionQualityCounts{}
	parseSessionSegments(map[string]string{
		"sessions\x1fsource\x1fnewsletter":  "3",
		"bounces\x1fsource\x1fnewsletter":   "1",
		"pageviews\x1fsource\x1fnewsletter": "8",
		"duration\x1fsource\x1fnewsletter":  "120",
		"invalid":                           "9",
	}, segments)
	quality := segments["source"]["newsletter"]
	if quality.Sessions != 3 || quality.Bounces != 1 || quality.Pageviews != 8 || quality.DurationSeconds != 120 {
		t.Fatalf("quality = %#v", quality)
	}
}

func TestParseCountsDropsNonPositiveValues(t *testing.T) {
	counts := map[string]int64{}
	parseCounts(map[string]string{"/": "2", "/old": "0", "/bad": "invalid"}, counts)
	if len(counts) != 1 || counts["/"] != 2 {
		t.Fatalf("counts = %#v", counts)
	}
}

func TestSafeSessionDimensionRemovesCompositeSeparator(t *testing.T) {
	if got := safeSessionDimension("/from\x1f/to"); got != "/from/to" {
		t.Fatalf("safe dimension = %q", got)
	}
}
