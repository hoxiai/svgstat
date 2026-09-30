package analytics

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoxiai/svgstat/internal/cache"
)

func TestTrackRequestData_TracksIPAndHourly(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	req1 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req1.RemoteAddr = "192.0.2.1:1234"

	req2 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req2.RemoteAddr = "192.0.2.2:5678"

	// Duplicate of req1 (same IP, same visitor)
	req3 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req3.RemoteAddr = "192.0.2.1:9999"

	// Same IP as req1, but new visitor
	req4 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req4.RemoteAddr = "192.0.2.1:8888"

	if err := a.TrackPageview(ctx, req1, projectID, "/test", "", "visitor-1", ""); err != nil {
		t.Fatalf("TrackPageview req1 error: %v", err)
	}
	if err := a.TrackPageview(ctx, req2, projectID, "/test", "", "visitor-2", ""); err != nil {
		t.Fatalf("TrackPageview req2 error: %v", err)
	}
	if err := a.TrackPageview(ctx, req3, projectID, "/test", "", "visitor-1", ""); err != nil {
		t.Fatalf("TrackPageview req3 error: %v", err)
	}
	if err := a.TrackPageview(ctx, req4, projectID, "/test", "", "visitor-3", ""); err != nil {
		t.Fatalf("TrackPageview req4 error: %v", err)
	}

	today, err := a.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if today.PV != 4 {
		t.Errorf("expected 4 PVs, got %d", today.PV)
	}
	if today.UV != 3 {
		t.Errorf("expected 3 unique visitors, got %d", today.UV)
	}
	if today.IP != 2 {
		t.Errorf("expected 2 unique IPs, got %d", today.IP)
	}

	if today.Hourly == nil {
		t.Fatalf("expected today.Hourly to be non-nil")
	}

	hour := time.Now().UTC().Format("15")
	if today.Hourly["pv"] == nil || today.Hourly["pv"][hour] != 4 {
		t.Errorf("expected hourly PV[%s] == 4, got %v", hour, today.Hourly["pv"])
	}
	if today.Hourly["uv"] == nil || today.Hourly["uv"][hour] != 3 {
		t.Errorf("expected hourly UV[%s] == 3, got %v", hour, today.Hourly["uv"])
	}
	if today.Hourly["ip"] == nil || today.Hourly["ip"][hour] != 2 {
		t.Errorf("expected hourly IP[%s] == 2, got %v", hour, today.Hourly["ip"])
	}
}

func TestTrackRequestData_BotsAndNonPageviewsIgnored(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	// Bot request
	botReq := httptest.NewRequest("GET", "https://example.com/test", nil)
	botReq.RemoteAddr = "192.0.2.100:1234"
	botReq.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	if err := a.TrackPageview(ctx, botReq, projectID, "/test", "", "bot-visitor", ""); err != nil {
		t.Fatalf("TrackPageview botReq error: %v", err)
	}

	// Plain request (not a pageview)
	plainReq := httptest.NewRequest("GET", "https://example.com/test", nil)
	plainReq.RemoteAddr = "192.0.2.101:5678"
	if err := a.TrackRequest(ctx, plainReq, projectID); err != nil {
		t.Fatalf("TrackRequest error: %v", err)
	}

	today, err := a.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if today.IP != 0 {
		t.Errorf("expected 0 IP, got %d", today.IP)
	}
	if today.PV != 0 {
		t.Errorf("expected 0 PV, got %d", today.PV)
	}
	if today.UV != 0 {
		t.Errorf("expected 0 UV, got %d", today.UV)
	}
	hour := time.Now().UTC().Format("15")
	if today.Hourly["pv"][hour] != 0 || today.Hourly["uv"][hour] != 0 || today.Hourly["ip"][hour] != 0 {
		t.Errorf("expected 0 hourly stats for bots and plain requests, got %v", today.Hourly)
	}
}

func TestTrackRequestData_KeysHaveTTL(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	req.RemoteAddr = "192.0.2.200:1234"
	if err := a.TrackPageview(ctx, req, projectID, "/test", "", "visitor-ttl", ""); err != nil {
		t.Fatalf("TrackPageview error: %v", err)
	}

	date := currentDate()
	hour := time.Now().UTC().Format("15")

	keysToCheck := []string{
		cache.BuildKey("project", projectID, "ipset", date),
		cache.BuildKey("project", projectID, "hourly", date, "pv"),
		cache.BuildKey("project", projectID, "hourly", date, "uv"),
		cache.BuildKey("project", projectID, "hourly", date, "ip"),
		cache.BuildKey("project", projectID, "hourly_uvset", date, hour),
		cache.BuildKey("project", projectID, "hourly_ipset", date, hour),
	}

	for _, k := range keysToCheck {
		ttl, err := a.cache.GetClient().TTL(ctx, k).Result()
		if err != nil {
			t.Errorf("error getting TTL for %s: %v", k, err)
			continue
		}
		if ttl <= 0 {
			t.Errorf("expected positive TTL for %s, got %v", k, ttl)
		}
	}
}

func TestGetTodayVisitors_MaskedIPPreserved(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	req.RemoteAddr = "114.240.12.34:1234"

	if err := a.TrackPageview(ctx, req, projectID, "/test", "", "visitor-1", ""); err != nil {
		t.Fatalf("TrackPageview error: %v", err)
	}

	visitors, err := a.GetTodayVisitors(ctx, projectID, VisitorQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("GetTodayVisitors error: %v", err)
	}
	if len(visitors.Items) != 1 {
		t.Fatalf("expected 1 visitor item, got %d", len(visitors.Items))
	}
	if visitors.Items[0].MaskedIP != "114.240.*.*" {
		t.Errorf("expected MaskedIP '114.240.*.*', got %q", visitors.Items[0].MaskedIP)
	}
}

func TestBuildVisitorDetail_MaskedIP(t *testing.T) {
	// Case 1: masked_ip present in data
	d1 := buildVisitorDetail("v1", map[string]string{
		"masked_ip": "114.240.*.*",
		"ip":        "anon_123456",
	})
	if d1.MaskedIP != "114.240.*.*" {
		t.Errorf("expected MaskedIP '114.240.*.*', got %q", d1.MaskedIP)
	}

	// Case 2: masked_ip absent, raw IP present
	d2 := buildVisitorDetail("v2", map[string]string{
		"ip": "114.240.12.34",
	})
	if d2.MaskedIP != "114.240.*.*" {
		t.Errorf("expected fallback MaskedIP '114.240.*.*', got %q", d2.MaskedIP)
	}

	// Case 3: masked_ip absent, anon IP present
	d3 := buildVisitorDetail("v3", map[string]string{
		"ip": "anon_abcdef123456",
	})
	if d3.MaskedIP != "" {
		t.Errorf("expected empty MaskedIP for anon IP without masked_ip, got %q", d3.MaskedIP)
	}

	// Case 4: JSON marshaling with and without MaskedIP
	b1, err := json.Marshal(d1)
	if err != nil {
		t.Fatalf("json.Marshal d1 failed: %v", err)
	}
	if !strings.Contains(string(b1), `"maskedIp":"114.240.*.*"`) {
		t.Errorf("expected json to contain maskedIp, got: %s", string(b1))
	}

	b3, err := json.Marshal(d3)
	if err != nil {
		t.Fatalf("json.Marshal d3 failed: %v", err)
	}
	if strings.Contains(string(b3), `"maskedIp"`) {
		t.Errorf("expected json to omit maskedIp when empty, got: %s", string(b3))
	}
}


