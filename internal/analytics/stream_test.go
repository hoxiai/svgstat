package analytics

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestVisitStream_PushAndRetrieve(t *testing.T) {
	a, projectID := newTestAnalytics(t)

	req := httptest.NewRequest("GET", "https://example.com/pricing", nil)
	req.RemoteAddr = "114.240.10.20:8080"
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://www.baidu.com/s?wd=svgstat%20counter")

	ctx := context.Background()
	_ = a.TrackPageview(ctx, req, projectID, "/pricing", "https://www.baidu.com/s?wd=svgstat%20counter", "vis-1", "")

	items, err := a.GetVisitStream(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stream item, got %d", len(items))
	}
	item := items[0]
	if item.MaskedIP != "114.240.*.*" {
		t.Errorf("expected masked IP, got %s", item.MaskedIP)
	}
	if item.SourceCategory != "search" || item.SourceName != "baidu" {
		t.Errorf("expected search/baidu source, got %s/%s", item.SourceCategory, item.SourceName)
	}
	if item.SearchKeyword != "svgstat counter" {
		t.Errorf("expected search keyword 'svgstat counter', got %s", item.SearchKeyword)
	}
	if item.Path != "/pricing" {
		t.Errorf("expected path '/pricing', got %s", item.Path)
	}
}

func TestVisitStream_BotNotTracked(t *testing.T) {
	a, projectID := newTestAnalytics(t)

	req := httptest.NewRequest("GET", "https://example.com/pricing", nil)
	req.RemoteAddr = "114.240.10.20:8080"
	req.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")

	ctx := context.Background()
	_ = a.TrackPageview(ctx, req, projectID, "/pricing", "", "vis-bot", "")

	items, err := a.GetVisitStream(ctx, projectID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 stream items for bot, got %d", len(items))
	}
}

func TestVisitStream_MaskIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"114.240.10.20", "114.240.*.*"},
		{"114.240.10.20:8080", "114.240.*.*"},
		{"192.168.1.1", "192.168.*.*"},
		{"2001:db8:85a3::8a2e:370:7334", "2001:db8:*::*"},
		{"anon_abcdef123456", "anon_*"},
		{"", ""},
	}

	for _, tt := range tests {
		got := maskIP(tt.input)
		if got != tt.expected {
			t.Errorf("maskIP(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestVisitStream_ClassifySourceCategory(t *testing.T) {
	tests := []struct {
		source   string
		medium   string
		expected string
	}{
		{"baidu", "organic", "search"},
		{"google", "organic", "search"},
		{"google", "cpc", "search"},
		{"chatgpt", "ai", "ai"},
		{"wechat", "social", "social"},
		{"github", "referral", "referral"},
		{"gmail", "email", "email"},
		{"direct", "none", "direct"},
		{"", "", "direct"},
	}

	for _, tt := range tests {
		got := classifySourceCategory(tt.source, tt.medium)
		if got != tt.expected {
			t.Errorf("classifySourceCategory(%q, %q) = %q; want %q", tt.source, tt.medium, got, tt.expected)
		}
	}
}

func TestVisitStream_LimitAndOrder(t *testing.T) {
	a, projectID := newTestAnalytics(t)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("https://example.com/page-%d", i), nil)
		req.RemoteAddr = fmt.Sprintf("192.168.1.%d:1234", i)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		_ = a.TrackPageview(ctx, req, projectID, fmt.Sprintf("/page-%d", i), "", fmt.Sprintf("vis-%d", i), "")
	}

	// Should return newest first with limit 3
	items, err := a.GetVisitStream(ctx, projectID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Path != "/page-5" {
		t.Errorf("expected newest path '/page-5', got %s", items[0].Path)
	}
	if items[1].Path != "/page-4" {
		t.Errorf("expected path '/page-4', got %s", items[1].Path)
	}
	if items[2].Path != "/page-3" {
		t.Errorf("expected path '/page-3', got %s", items[2].Path)
	}
}
