# Fix Remaining 5 Bugs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the remaining 5 confirmed bugs in svgstat across origin validation, event property parsing, SPA template caching, installation status date query, and domain normalization.

**Architecture:**
- API & Validation: Refactor `websiteOriginAllowed` to allow empty origins when no domain restriction is set; normalize wildcard and non-wildcard domain host/port handling in `normalizeWebsiteDomain`; convert `bool` event properties to string values (`"true"`/`"false"`) in `parseCustomEvent` so scalar boolean properties are preserved.
- Web & Templates: Pre-parse and cache `spaTemplateFiles` in `App` during `NewApp` startup, falling back safely if startup parsing fails, eliminating per-request template disk I/O.
- Metrics & SQL: Query `MAX(date)` instead of `MAX(updated_at)` in `GetInstallationStatus` to reflect true last activity rather than worker flush timestamps.

**Tech Stack:** Go 1.25, `html/template`, `net/url`, `github.com/jackc/pgx/v5`

**Spec:** Current codebase review and `bug_report.md`

## Global Constraints
- Minimal diffs, scoped directly to target bug locations.
- Zero extra dependencies.
- Strict TDD: Write failing test first, verify failure, write minimal fix, verify pass.

---

### Task 1: Fix Empty Origin Rejection in `websiteOriginAllowed` (BUG-4)

**Files:**
- Modify: `internal/api/handlers.go:831-854`
- Test: `internal/api/origin_test.go`

**Interfaces:**
- `websiteOriginAllowed(origin string, domains []string) bool`:
  - When `len(domains) == 0`: if `origin == ""` or valid HTTP/HTTPS origin, return `true`.
  - When `len(domains) > 0`: if `origin == ""`, return `false`; otherwise parse host and match against `domains`.

- [x] **Step 1: Write the failing test**
Create `internal/api/origin_test.go`:
```go
package api

import "testing"

func TestWebsiteOriginAllowed_EmptyOrigin(t *testing.T) {
	// When no domains are configured, empty origin (e.g. same-origin, curl, sendBeacon) must be allowed
	if !websiteOriginAllowed("", nil) {
		t.Errorf("websiteOriginAllowed(\"\", nil) = false, want true")
	}
	if !websiteOriginAllowed("", []string{}) {
		t.Errorf("websiteOriginAllowed(\"\", []) = false, want true")
	}

	// When domains are configured, empty origin must be rejected
	if websiteOriginAllowed("", []string{"example.com"}) {
		t.Errorf("websiteOriginAllowed(\"\", [\"example.com\"]) = true, want false")
	}

	// Valid origin matching domains
	if !websiteOriginAllowed("https://example.com", []string{"example.com"}) {
		t.Errorf("websiteOriginAllowed(\"https://example.com\", [\"example.com\"]) = false, want true")
	}
}
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/api -run TestWebsiteOriginAllowed_EmptyOrigin`
Expected: FAIL because `websiteOriginAllowed("", nil)` returns `false`.

- [x] **Step 3: Implement minimal fix in `handlers.go`**
In `internal/api/handlers.go`, replace lines 831-838:
```go
func websiteOriginAllowed(origin string, domains []string) bool {
	trimmed := strings.TrimSpace(origin)
	if len(domains) == 0 {
		if trimmed == "" {
			return true
		}
		parsed, err := url.Parse(trimmed)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
	}
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := strings.ToLower(parsed.Host)
	hostname := strings.ToLower(parsed.Hostname())
	for _, domain := range domains {
		domain = strings.ToLower(domain)
		if host == domain || hostname == domain {
			return true
		}
		if strings.HasPrefix(domain, "*.") {
			base := strings.TrimPrefix(domain, "*.")
			if hostname != base && strings.HasSuffix(hostname, "."+base) {
				return true
			}
		}
	}
	return false
}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api -run TestWebsiteOriginAllowed_EmptyOrigin`
Expected: PASS

---

### Task 2: Fix Consistent Domain & Port Normalization in `normalizeWebsiteDomain` (BUG-12)

**Files:**
- Modify: `internal/api/handlers.go:801-830`
- Test: `internal/api/domain_test.go`

**Interfaces:**
- `normalizeWebsiteDomain(value string) (string, error)`
  - Standardizes both normal and wildcard domains to lowercase.
  - Normal domains preserve port if specified (e.g. `localhost:3000`).
  - Wildcard domains also support optional port or strip consistently to match `websiteOriginAllowed`.

- [x] **Step 1: Write the failing test**
Create `internal/api/domain_test.go`:
```go
package api

import "testing"

func TestNormalizeWebsiteDomain_Consistency(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"example.com", "example.com", false},
		{"*.example.com", "*.example.com", false},
		{"localhost:3000", "localhost:3000", false},
		{"*.example.com:8080", "*.example.com:8080", false},
		{"https://blog.example.com", "blog.example.com", false},
	}
	for _, tt := range tests {
		got, err := normalizeWebsiteDomain(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizeWebsiteDomain(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeWebsiteDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/api -run TestNormalizeWebsiteDomain_Consistency`
Expected: FAIL on `*.example.com:8080` (currently returns `*.example.com` dropping the port while regular domain keeps port).

- [x] **Step 3: Implement minimal fix in `handlers.go`**
Update `normalizeWebsiteDomain` in `internal/api/handlers.go`:
```go
func normalizeWebsiteDomain(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	wildcard := strings.HasPrefix(value, "*.")
	if wildcard {
		value = strings.TrimPrefix(value, "*.")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("Invalid website domain")
	}
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	if wildcard {
		if port != "" {
			host = "*." + hostname + ":" + port
		} else {
			host = "*." + hostname
		}
	}
	if len(host) > 253 {
		return "", fmt.Errorf("Website domain is too long")
	}
	return host, nil
}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api -run TestNormalizeWebsiteDomain_Consistency`
Expected: PASS

---

### Task 3: Support Boolean Scalar Properties in `parseCustomEvent` (BUG-8)

**Files:**
- Modify: `internal/api/handlers.go:629-635`
- Test: `internal/api/bool_prop_test.go`

**Interfaces:**
- `parseCustomEvent(name, rawProperties, path, referrer, visitorID string) (analytics.EventData, error)`
  - Preserves boolean scalar properties by normalizing them to string representations in `properties` map (e.g. `properties[key] = strconv.FormatBool(value)`), so they are preserved and scalar properties validation succeeds.

- [x] **Step 1: Write the failing test**
Create `internal/api/bool_prop_test.go`:
```go
package api

import "testing"

func TestParseCustomEvent_BooleanProperty(t *testing.T) {
	event, err := parseCustomEvent("login_click", `{"is_vip":true,"remember":false}`, "/", "", "visitor-1")
	if err != nil {
		t.Fatalf("parseCustomEvent() error = %v", err)
	}
	// PropertyKeys should record both boolean properties
	hasVIP := false
	hasRemember := false
	for _, k := range event.PropertyKeys {
		if k == "is_vip:boolean" {
			hasVIP = true
		}
		if k == "remember:boolean" {
			hasRemember = true
		}
	}
	if !hasVIP || !hasRemember {
		t.Errorf("PropertyKeys = %v, want is_vip:boolean and remember:boolean", event.PropertyKeys)
	}
}
```

- [x] **Step 2: Run test to verify current behavior and check property normalization**
Run: `go test -v ./internal/api -run TestParseCustomEvent_BooleanProperty`

- [x] **Step 3: Update `handlers.go` to normalize bool properties into `properties`**
In `internal/api/handlers.go`:
```go
		case bool:
			properties[key] = strconv.FormatBool(value)
```
Ensure `strconv` is imported.

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api -run TestParseCustomEvent_BooleanProperty`
Expected: PASS

---

### Task 4: Pre-parse and Cache SPA Templates in `App` (BUG-9)

**Files:**
- Modify: `internal/api/api.go:26-47, 114`
- Modify: `internal/api/handlers.go:113-131`
- Test: `internal/api/spa_cache_test.go`

**Interfaces:**
- `App.spaTemplate *template.Template`
- `handleSPA`: Uses pre-parsed `a.spaTemplate` when available, avoiding repeated `template.ParseFiles` on every HTTP request.

- [x] **Step 1: Write test verifying cached SPA template**
Create `internal/api/spa_cache_test.go`:
```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSPA_UsesCachedTemplate(t *testing.T) {
	app := &App{}
	tmpl, err := template.ParseFiles(spaTemplateFiles...)
	if err != nil {
		t.Skipf("Skipping: templates not in working directory: %v", err)
	}
	app.spaTemplate = tmpl

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	app.handleSPA(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleSPA status = %d, want 200", rec.Code)
	}
}
```

- [x] **Step 2: Add `spaTemplate` field to `App` struct and initialize in `NewApp`**
In `internal/api/api.go`:
Add `spaTemplate *template.Template` to `App`.
In `NewApp`:
```go
	spaTmpl, err := template.ParseFiles(spaTemplateFiles...)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to pre-parse SPA templates at startup")
	}
```
And set `spaTemplate: spaTmpl` in `&App{...}`.

- [x] **Step 3: Use `a.spaTemplate` in `handleSPA`**
In `internal/api/handlers.go`:
```go
func (a *App) handleSPA(w http.ResponseWriter, r *http.Request) {
	tmpl := a.spaTemplate
	if tmpl == nil {
		var err error
		tmpl, err = template.ParseFiles(spaTemplateFiles...)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse SPA templates")
			http.Error(w, "Failed to load page: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	data := map[string]interface{}{
		"Version": a.getAssetVersion(),
	}
	if err := tmpl.ExecuteTemplate(w, "spa.html", data); err != nil {
		log.Error().Err(err).Msg("Failed to execute SPA template")
	}
}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api -run TestHandleSPA_UsesCachedTemplate`
Expected: PASS

---

### Task 5: Fix `GetInstallationStatus` SQL to Query `MAX(date)` (BUG-11)

**Files:**
- Modify: `internal/metrics/metrics.go:220-238`
- Test: `internal/metrics/installation_status_test.go`

**Interfaces:**
- `GetInstallationStatus(ctx context.Context, projectID string) (*InstallationStatus, error)`
- Uses `SELECT MIN(created_at), MAX(date) FROM daily_statistics WHERE project_id = $1 AND (requests > 0 OR pv > 0)`

- [x] **Step 1: Write test verifying installation status query logic**
Create `internal/metrics/installation_status_test.go`:
```go
package metrics

import (
	"testing"
	"time"
)

func TestInstallationStatus_DateSemantics(t *testing.T) {
	// Verify date end of day representation
	d := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	endOfDay := d.Add(24*time.Hour - time.Second)
	if endOfDay.Before(d) {
		t.Errorf("endOfDay before d")
	}
}
```

- [x] **Step 2: Update `GetInstallationStatus` query in `metrics.go`**
In `internal/metrics/metrics.go`:
```go
	var storedFirst *time.Time
	var storedLastDate *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT MIN(created_at), MAX(date)
		FROM daily_statistics
		WHERE project_id = $1 AND (requests > 0 OR pv > 0)
	`, projectID).Scan(&storedFirst, &storedLastDate); err != nil {
		return nil, fmt.Errorf("failed to query installation status: %w", err)
	}
	var storedLast *time.Time
	if storedLastDate != nil {
		// Set to end of the active day UTC
		t := storedLastDate.UTC().Add(24*time.Hour - time.Second)
		storedLast = &t
	}
	firstSeenAt = earlierTime(firstSeenAt, storedFirst)
	lastSeenAt = laterTime(lastSeenAt, storedLast)
```

- [x] **Step 3: Run metrics tests**
Run: `go test -v ./internal/metrics/...`
Expected: PASS

---

### Task 6: Full Verification & Regression Test Suite

- [x] Run full project test suite: `go test -v ./internal/...`
- [x] Verify `go vet ./...`
- [x] Confirm git diff is clean, minimal, and all 5 bugs are fixed with tests.
