package analytics

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
	"github.com/hoxiai/svgstat/internal/config"
	"github.com/joho/godotenv"
)

func newTestAnalytics(t *testing.T) (*Analytics, string) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	c, err := cache.New(config.Load())
	if err != nil {
		t.Skip("Skipping test: Redis not reachable")
	}
	projectID := fmt.Sprintf("test-attr-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		keys, _ := c.GetClient().Keys(context.Background(), cache.BuildKey("project", projectID, "*")).Result()
		if len(keys) > 0 {
			_ = c.Delete(context.Background(), keys...)
		}
		c.Close()
	})
	return New(c, nil, nil, time.Hour, "salt"), projectID
}

func trackTestPageview(t *testing.T, a *Analytics, projectID, visitor, path, referrer, search string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/collect", nil)
	req.Header.Set("Origin", "https://mysite.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	req.RemoteAddr = "203.0.113.7:1234"
	if err := a.TrackPageview(context.Background(), req, projectID, path, referrer, visitor, search); err != nil {
		t.Fatalf("TrackPageview() error = %v", err)
	}
}

func TestSourcesCountOncePerVisit(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	google := "https://www.google.com/search?q=svg+badge"
	trackTestPageview(t, a, projectID, "visitor-a", "/", google, "")
	trackTestPageview(t, a, projectID, "visitor-a", "/docs", google, "")
	trackTestPageview(t, a, projectID, "visitor-b", "/?utm_source=newsletter&utm_medium=email&utm_term=launch", "", "")

	stats, err := a.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats() error = %v", err)
	}
	if stats.PV != 3 || stats.Sessions != 2 {
		t.Fatalf("PV/Sessions = %d/%d, want 3/2", stats.PV, stats.Sessions)
	}
	want := map[string]map[string]int64{
		"sources":   {"google": 1, "newsletter": 1},
		"mediums":   {"organic": 1, "email": 1},
		"referrers": {"https://www.google.com/search": 1},
		"channels":  {"organic" + compositeSeparator + "google": 1, "email" + compositeSeparator + "newsletter": 1},
		"terms":     {"svg badge": 1, "launch": 1},
	}
	got := map[string]map[string]int64{
		"sources": stats.Sources, "mediums": stats.Mediums, "referrers": stats.Referrers,
		"channels": stats.Channels, "terms": stats.Terms,
	}
	for name, values := range want {
		if fmt.Sprint(got[name]) != fmt.Sprint(values) {
			t.Errorf("%s = %v, want %v", name, got[name], values)
		}
	}
}

func TestSiteSearchCountsEverySearch(t *testing.T) {
	a, projectID := newTestAnalytics(t)

	trackTestPageview(t, a, projectID, "visitor-a", "/search?q=Pricing", "", "Pricing")
	trackTestPageview(t, a, projectID, "visitor-a", "/search?q=api", "", "api")
	trackTestPageview(t, a, projectID, "visitor-b", "/search?q=pricing", "", "pricing")
	trackTestPageview(t, a, projectID, "visitor-b", "/about", "", "")

	stats, err := a.GetTodayStats(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetTodayStats() error = %v", err)
	}
	if fmt.Sprint(stats.SiteSearches) != fmt.Sprint(map[string]int64{"api": 1, "pricing": 2}) {
		t.Fatalf("SiteSearches = %v, want api:1 pricing:2", stats.SiteSearches)
	}
}

func TestRealtimeCountsVisitorsBySourceAndCurrentPage(t *testing.T) {
	a, projectID := newTestAnalytics(t)

	trackTestPageview(t, a, projectID, "visitor-a", "/", "https://www.google.com/", "")
	trackTestPageview(t, a, projectID, "visitor-a", "/docs", "https://www.google.com/", "")
	trackTestPageview(t, a, projectID, "visitor-b", "/pricing", "", "")

	realtime, err := a.GetRealtimeStats(context.Background(), projectID, time.Now())
	if err != nil {
		t.Fatalf("GetRealtimeStats() error = %v", err)
	}
	if fmt.Sprint(realtime.Sources) != fmt.Sprint(map[string]int64{"direct": 1, "google": 1}) {
		t.Errorf("Sources = %v, want direct:1 google:1", realtime.Sources)
	}
	if fmt.Sprint(realtime.Pages) != fmt.Sprint(map[string]int64{"/docs": 1, "/pricing": 1}) {
		t.Errorf("Pages = %v, want /docs:1 /pricing:1", realtime.Pages)
	}
}
