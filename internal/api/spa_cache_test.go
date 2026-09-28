package api

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSPA_UsesCachedTemplate(t *testing.T) {
	tmpl, err := template.New("spa.html").Parse("<!DOCTYPE html><html><body>Version: {{.Version}}</body></html>")
	if err != nil {
		t.Fatalf("template.Parse: %v", err)
	}
	app := &App{
		spaTemplate: tmpl,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	app.handleSPA(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleSPA status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Version:") {
		t.Errorf("expected body to contain 'Version:', got %s", body)
	}
}
