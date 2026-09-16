package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/hoxiai/svgstat/internal/observability"
)

func TestSPACacheHeadersAndVersion(t *testing.T) {
	if _, err := os.Stat("web/spa.html"); err != nil {
		if _, err := os.Stat("../../web/spa.html"); err == nil {
			orig, _ := os.Getwd()
			_ = os.Chdir("../..")
			defer func() { _ = os.Chdir(orig) }()
		}
	}

	app := &App{}
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	app.handleSPA(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc == "" || !regexp.MustCompile(`(?i)no-cache.*no-store|no-store.*no-cache`).MatchString(cc) {
		t.Errorf("expected handleSPA Cache-Control to include no-cache and no-store, got: %q", cc)
	}

	body := rec.Body.String()

	// Must contain version query string for css and js
	cssVersionPattern := regexp.MustCompile(`/static/css/style\.css\?v=([a-zA-Z0-9_-]+)`)
	if !cssVersionPattern.MatchString(body) {
		t.Errorf("expected style.css to have version query param ?v=..., rendered HTML was:\n%s", body[:min(len(body), 500)])
	}

	jsVersionPattern := regexp.MustCompile(`/static/js/app\.js\?v=([a-zA-Z0-9_-]+)`)
	if !jsVersionPattern.MatchString(body) {
		t.Errorf("expected app.js to have version query param ?v=..., rendered HTML was:\n%s", body[:min(len(body), 500)])
	}
}

func TestStaticRouteHasNoCacheHeader(t *testing.T) {
	if _, err := os.Stat("web/spa.html"); err != nil {
		if _, err := os.Stat("../../web/spa.html"); err == nil {
			orig, _ := os.Getwd()
			_ = os.Chdir("../..")
			defer func() { _ = os.Chdir(orig) }()
		}
	}

	app := &App{
		observability: observability.New(),
	}
	router := app.SetupRoutes()

	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for static file, got %d", rec.Code)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected static file Cache-Control to be 'no-cache', got: %q", cc)
	}
}
