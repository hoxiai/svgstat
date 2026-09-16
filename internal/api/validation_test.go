package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/svgstat/svgstat/internal/config"
	"github.com/svgstat/svgstat/internal/requestmeta"
)

func TestParseCustomEvent(t *testing.T) {
	event, err := parseCustomEvent(" Purchase.Completed ", `{"value":99.5,"currency":"cny","coupon":"launch","first":true}`, "/checkout?utm_source=newsletter", "https://example.com", "visitor-1")
	if err != nil {
		t.Fatalf("parseCustomEvent() error = %v", err)
	}
	if event.Name != "purchase.completed" || event.Value == nil || *event.Value != 99.5 || event.Currency != "CNY" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if _, err := parseCustomEvent("bad name", "", "/", "", ""); err == nil {
		t.Fatal("invalid event name was accepted")
	}
	if _, err := parseCustomEvent("purchase", `{"value":-1}`, "/", "", ""); err == nil {
		t.Fatal("negative value was accepted")
	}
	nested, _ := json.Marshal(map[string]interface{}{"nested": map[string]string{"bad": "value"}})
	if _, err := parseCustomEvent("purchase", string(nested), "/", "", ""); err == nil {
		t.Fatal("nested properties were accepted")
	}
}

func TestTestModeNeverWritesAnalytics(t *testing.T) {
	if shouldWriteAnalytics("test") {
		t.Fatal("test mode would write production analytics")
	}
	if !shouldWriteAnalytics("live") {
		t.Fatal("live mode would skip production analytics")
	}
}

func TestAutomaticEventDimensionsWhitelistSafeProperties(t *testing.T) {
	dimensions := automaticEventDimensions("file_download", map[string]interface{}{
		"file_extension": "pdf",
		"target_domain":  "docs.example",
		"target_path":    "/guide.pdf?token=private#section",
		"email":          "private@example.com",
	})
	if len(dimensions) != 3 || dimensions["file_extension"] != "pdf" || dimensions["target_domain"] != "docs.example" || dimensions["target_path"] != "/guide.pdf" {
		t.Fatalf("dimensions = %#v", dimensions)
	}
	if _, ok := dimensions["email"]; ok {
		t.Fatal("non-whitelisted event property was aggregated")
	}
	if got := automaticEventDimensions("purchase", map[string]interface{}{"target_domain": "example.com"}); len(got) != 0 {
		t.Fatalf("manual event dimensions = %#v", got)
	}
	if got := automaticEventDimensions("contact_click", map[string]interface{}{"contact_type": "private@example.com"}); len(got) != 0 {
		t.Fatalf("unsafe contact dimensions = %#v", got)
	}
	if got := automaticEventDimensions("outbound_click", map[string]interface{}{"target_domain": "person@example.com"}); len(got) != 0 {
		t.Fatalf("unsafe target dimensions = %#v", got)
	}
	if got := automaticEventDimensions("web_vital_lcp", map[string]interface{}{"rating": "poor", "selector": "#private"}); len(got) != 1 || got["rating"] != "poor" {
		t.Fatalf("web vital dimensions = %#v", got)
	}
	if got := automaticEventDimensions("js_error", map[string]interface{}{"error_type": "TypeError: private message"}); len(got) != 0 {
		t.Fatalf("unsafe error dimensions = %#v", got)
	}
}

func TestParseWebVitalValidatesAndRecomputesRating(t *testing.T) {
	event, err := parseCustomEvent("web_vital_lcp", `{"value":3200,"rating":"good"}`, "/", "", "visitor-1")
	if err != nil {
		t.Fatalf("parseCustomEvent() error = %v", err)
	}
	if event.Value == nil || *event.Value != 3200 || event.Dimensions["rating"] != "needs_improvement" {
		t.Fatalf("event = %#v", event)
	}
	if _, err := parseCustomEvent("web_vital_cls", `{"rating":"good"}`, "/", "", "visitor-1"); err == nil {
		t.Fatal("web vital without a value was accepted")
	}
	if _, err := parseCustomEvent("web_vital_inp", `{"value":70000}`, "/", "", "visitor-1"); err == nil {
		t.Fatal("out-of-range web vital was accepted")
	}
}

func TestValidateRegistration(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{name: "valid", email: "user@example.com", password: "password123"},
		{name: "bad email", email: "not-an-email", password: "password123", wantErr: true},
		{name: "short password", email: "user@example.com", password: "short", wantErr: true},
		{name: "bcrypt limit", email: "user@example.com", password: string(make([]byte, 73)), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRegistration(test.email, test.password, "Name")
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRegistration() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateProject(t *testing.T) {
	tests := []struct {
		slug    string
		wantErr bool
	}{
		{slug: "my-project"},
		{slug: "a"},
		{slug: "Uppercase", wantErr: true},
		{slug: "-leading", wantErr: true},
		{slug: "space here", wantErr: true},
	}

	for _, test := range tests {
		err := validateProject("Project", test.slug, "")
		if (err != nil) != test.wantErr {
			t.Errorf("validateProject(slug=%q) error = %v, wantErr %v", test.slug, err, test.wantErr)
		}
	}
}

func TestValidateSVGRequest(t *testing.T) {
	request := httptest.NewRequest("GET", "/svg/demo/badge/request.svg?label=ok&page_id=repo", nil)
	if err := validateSVGRequest(request, "request.count"); err != nil {
		t.Fatalf("valid SVG request rejected: %v", err)
	}
	if err := validateSVGRequest(request, "bad@metric"); err == nil {
		t.Fatal("invalid metric name was accepted")
	}
	invalidPreview := httptest.NewRequest("GET", "/svg/demo/badge/request.svg?preview=yes", nil)
	if err := validateSVGRequest(invalidPreview, "request"); err == nil {
		t.Fatal("invalid preview value was accepted")
	}
}

func TestIsPreviewRequest(t *testing.T) {
	if !isPreviewRequest(httptest.NewRequest("GET", "/svg/demo/badge/request.svg?preview=1", nil)) {
		t.Fatal("preview request was not recognized")
	}
	if isPreviewRequest(httptest.NewRequest("GET", "/svg/demo/badge/request.svg", nil)) {
		t.Fatal("regular request was recognized as preview")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	app := &App{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := app.csrfMiddleware(next)

	tests := []struct {
		name   string
		origin string
		cookie bool
		want   int
	}{
		{name: "same origin cookie", origin: "https://example.com", cookie: true, want: http.StatusNoContent},
		{name: "cross origin cookie", origin: "https://evil.example", cookie: true, want: http.StatusForbidden},
		{name: "missing origin cookie", cookie: true, want: http.StatusForbidden},
		{name: "no cookie API client", want: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "https://example.com/api/v1/projects", nil)
			request.Host = "example.com"
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: "session_token", Value: "token"})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestCSRFMiddlewareAllowsPublicCollection(t *testing.T) {
	app := &App{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest("POST", "https://stats.example/api/v1/collect", nil)
	request.Header.Set("Origin", "https://site.example")
	request.AddCookie(&http.Cookie{Name: "session_token", Value: "token"})
	response := httptest.NewRecorder()
	app.csrfMiddleware(next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestRateLimiterResetsWindow(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute, "test", nil)
	now := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if !limiter.allow(context.Background(), "client", now) || !limiter.allow(context.Background(), "client", now) {
		t.Fatal("requests within limit were rejected")
	}
	if limiter.allow(context.Background(), "client", now) {
		t.Fatal("request over limit was accepted")
	}
	if !limiter.allow(context.Background(), "client", now.Add(time.Minute)) {
		t.Fatal("limiter did not reset after the window")
	}
}

func TestSameOriginHonorsForwardedHTTPS(t *testing.T) {
	request := httptest.NewRequest("POST", "http://example.com/api/v1/projects", nil)
	request.Host = "example.com"
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Forwarded-Proto", "https")
	source, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{requestMeta: requestmeta.NewResolver([]string{"192.0.2.1"})}
	if !app.sameOrigin(source, request) {
		t.Fatal("forwarded HTTPS request was rejected")
	}
}

func TestSameOriginAllowsPortMismatch(t *testing.T) {
	request := httptest.NewRequest("POST", "http://example.com:8080/api/v1/projects", nil)
	request.Host = "example.com:8080"
	source, err := url.Parse("http://example.com")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	if !app.sameOrigin(source, request) {
		t.Fatal("port-mismatched same-host request was rejected")
	}
}

func TestCSRFMiddlewareCanBeDisabled(t *testing.T) {
	app := &App{
		config: &config.Config{
			HTTP: config.HTTPConfig{CSRFCheckEnabled: false},
		},
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest("POST", "https://example.com/api/v1/projects", nil)
	request.Host = "example.com"
	request.Header.Set("Origin", "https://evil.example")
	request.AddCookie(&http.Cookie{Name: "session_token", Value: "token"})

	response := httptest.NewRecorder()
	app.csrfMiddleware(next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("disabled CSRF check rejected request, code = %d", response.Code)
	}
}

func TestNormalizeWebsiteDomains(t *testing.T) {
	domains, err := normalizeWebsiteDomains([]string{" HTTPS://Example.COM ", "*.Docs.Example.com", "localhost:3000", "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "*.docs.example.com", "localhost:3000"}
	if len(domains) != len(want) {
		t.Fatalf("domains = %#v, want %#v", domains, want)
	}
	for index := range want {
		if domains[index] != want[index] {
			t.Fatalf("domains[%d] = %q, want %q", index, domains[index], want[index])
		}
	}
	if _, err := normalizeWebsiteDomains([]string{"https://example.com/path"}); err == nil {
		t.Fatal("domain with a path was accepted")
	}
}

func TestWebsiteOriginAllowed(t *testing.T) {
	tests := []struct {
		origin  string
		domains []string
		want    bool
	}{
		{origin: "https://example.com", want: true},
		{origin: "https://example.com", domains: []string{"example.com"}, want: true},
		{origin: "https://app.example.com", domains: []string{"*.example.com"}, want: true},
		{origin: "https://example.com", domains: []string{"*.example.com"}},
		{origin: "https://evil.example", domains: []string{"example.com"}},
		{origin: "null"},
	}
	for _, test := range tests {
		if got := websiteOriginAllowed(test.origin, test.domains); got != test.want {
			t.Errorf("websiteOriginAllowed(%q, %#v) = %v, want %v", test.origin, test.domains, got, test.want)
		}
	}
}

func TestValidateCollectionEvent(t *testing.T) {
	if err := validateCollectionEvent("my-site", "/docs/start", "https://example.com/", "visitor-id"); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	for _, test := range []struct {
		name      string
		project   string
		path      string
		visitorID string
	}{
		{name: "bad project", project: "Bad Key", path: "/"},
		{name: "relative path", project: "site", path: "docs"},
		{name: "bad visitor", project: "site", path: "/", visitorID: "bad visitor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateCollectionEvent(test.project, test.path, "", test.visitorID); err == nil {
				t.Fatal("invalid event was accepted")
			}
		})
	}
}

func TestSetCollectionCORSHeaders(t *testing.T) {
	response := httptest.NewRecorder()
	setCollectionCORSHeaders(response, "https://example.com")
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("allow origin = %q", got)
	}
}
