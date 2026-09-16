package analytics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractRequestDataAnonymizesIP(t *testing.T) {
	analytics := New(nil, nil, nil, 0, "test-salt")
	request := httptest.NewRequest("GET", "http://example.com/svg/demo/counter/visits.svg", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")

	data := analytics.extractRequestData(request, "demo")
	if data.IP == "203.0.113.10" || !strings.HasPrefix(data.IP, "anon_") {
		t.Fatalf("IP was not anonymized: %q", data.IP)
	}
	if data.IP != analytics.anonymizeIP("203.0.113.10") {
		t.Fatalf("anonymized IP is not stable: %q", data.IP)
	}
}

func TestAnonymizeIPUsesDeploymentSalt(t *testing.T) {
	first := New(nil, nil, nil, 0, "first").anonymizeIP("203.0.113.10")
	second := New(nil, nil, nil, 0, "second").anonymizeIP("203.0.113.10")
	if first == second {
		t.Fatalf("different salts produced the same identifier: %q", first)
	}
}

func TestParseOptionalTime(t *testing.T) {
	want := time.Date(2026, 8, 24, 3, 4, 5, 0, time.UTC)
	got := parseOptionalTime(want.Format(time.RFC3339))
	if got == nil || !got.Equal(want) {
		t.Fatalf("parseOptionalTime() = %v, want %v", got, want)
	}
	if parseOptionalTime("invalid") != nil || parseOptionalTime(nil) != nil {
		t.Fatal("invalid installation time was accepted")
	}
}

func TestWebsiteAttribution(t *testing.T) {
	path, referrer, source, medium, campaign := websiteAttribution(
		"/pricing?utm_source=Newsletter&utm_medium=Email&utm_campaign=Summer%20Launch#plans",
		"https://search.example/results?q=svg",
	)
	if path != "/pricing#plans" || referrer != "https://search.example/results" {
		t.Fatalf("path/referrer = %q / %q", path, referrer)
	}
	if source != "newsletter" || medium != "email" || campaign != "summer launch" {
		t.Fatalf("attribution = %q / %q / %q", source, medium, campaign)
	}
}

func TestWebsiteAttributionInfersDirectAndReferral(t *testing.T) {
	_, _, source, medium, _ := websiteAttribution("/", "")
	if source != "direct" || medium != "none" {
		t.Fatalf("direct attribution = %q / %q", source, medium)
	}
	_, _, source, medium, _ = websiteAttribution("/docs", "https://github.com/org/repo")
	if source != "github.com" || medium != "referral" {
		t.Fatalf("referral attribution = %q / %q", source, medium)
	}

	// Search engine -> organic
	_, _, source, medium, _ = websiteAttribution("/", "https://www.google.com/search?q=svgstat")
	if source != "www.google.com" || medium != "organic" {
		t.Fatalf("google attribution = %q / %q, want organic", source, medium)
	}

	_, _, source, medium, _ = websiteAttribution("/", "https://www.baidu.com/s?wd=svgstat")
	if source != "www.baidu.com" || medium != "organic" {
		t.Fatalf("baidu attribution = %q / %q, want organic", source, medium)
	}

	// Social media -> social
	_, _, source, medium, _ = websiteAttribution("/", "https://x.com/user/status/123")
	if source != "x.com" || medium != "social" {
		t.Fatalf("x attribution = %q / %q, want social", source, medium)
	}

	_, _, source, medium, _ = websiteAttribution("/", "https://www.zhihu.com/question/123")
	if source != "www.zhihu.com" || medium != "social" {
		t.Fatalf("zhihu attribution = %q / %q, want social", source, medium)
	}
}

func TestCleanReferrerRemovesQueryAndFragment(t *testing.T) {
	got := cleanReferrer("https://example.com/search?q=secret#result")
	if got != "https://example.com/search" {
		t.Fatalf("cleanReferrer() = %q", got)
	}
}

func TestShouldCountPageview(t *testing.T) {
	if shouldCountPageview(false, false) {
		t.Fatal("SVG request was counted as a website page view")
	}
	if !shouldCountPageview(false, true) {
		t.Fatal("website event was not counted as a page view")
	}
	if shouldCountPageview(true, true) {
		t.Fatal("bot event was counted as a page view")
	}
}
