package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandleSPARendersEmbeddedComponents(t *testing.T) {
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

	body := rec.Body.String()

	expectedSnippets := []string{
		`id="navbar-container"`,
		`class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8"`, // from Navbar.html
		`currentPage === 'home'`,                         // from Home.html
		`currentPage === 'login'`,                        // from Login.html
		`currentPage === 'register'`,                     // from Register.html
		`currentPage === 'dashboard'`,                    // from Dashboard.html
		`currentPage === 'project-detail'`,               // from DashboardProject.html
		`projectTab === 'overview'`,                      // from TabOverview.html
		`projectTab === 'growth'`,                        // from TabGrowth.html
		`projectTab === 'quality'`,                       // from TabQuality.html
		`projectTab === 'visitors'`,                      // from TabVisitors.html
		`projectTab === 'diagnostics'`,                   // from TabDiagnostics.html
		`showCreateModal`,                                // from CreateProjectModal.html
		`showCodeModal`,                                  // from ProjectDetailModal.html
		`/static/vendor/unocss-runtime.js`,
		`type="module" src="/static/js/app.js?v=`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(body, snippet) {
			t.Errorf("rendered SPA HTML missing expected component snippet: %s", snippet)
		}
	}
}

func TestStaticAssetsAvailable(t *testing.T) {
	if _, err := os.Stat("web/spa.html"); err != nil {
		if _, err := os.Stat("../../web/spa.html"); err == nil {
			orig, _ := os.Getwd()
			_ = os.Chdir("../..")
			defer func() { _ = os.Chdir(orig) }()
		}
	}

	files := []string{
		"web/static/vendor/alpine.esm.js",
		"web/static/vendor/unocss-runtime.js",
		"web/static/js/app.js",
		"web/static/js/translations.js",
		"web/static/js/modules/state.js",
		"web/static/js/modules/formatters.js",
		"web/static/js/modules/router.js",
		"web/static/js/modules/auth.js",
		"web/static/js/modules/projects.js",
		"web/static/js/modules/analytics.js",
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("expected static file to exist: %s, error: %v", file, err)
		} else if info.Size() == 0 {
			t.Errorf("expected static file to not be empty: %s", file)
		}
	}
}
