# Fix Confirmed Bugs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Fix 6 confirmed functional/data bugs and 1 frontend sequence issue across daily metrics comparison, domain preview URLs, breakdown truncation, scheme-less referrer path parsing, visitor IP display, AI visits PostgreSQL persistence, and navigation mode switching.

**Architecture:**
- Metrics & SQL: In `today_overview.go`, accurately scale `yesterdaySamePeriod.UV` and `IP` using yesterday's hourly distribution scaled by `yesterdayFull` deduplicated total to prevent false negative drops; increase `breakdownLimits["paths"]` to 100 in `metrics.go` so the searchable pages table can search and paginate beyond 20 pages; add migration `026_add_daily_statistics_ai_visits.sql` and worker persistence in `worker.go` to persist `ai_visits`.
- Analytics & API: In `analytics.go`, normalize scheme-less referrers before URL parsing in `TrackBadgeOrCounter` to prevent hostnames from polluting path statistics; store `masked_ip` in `visitor_v2` and expose it on `VisitorDetail` so the IP table shows `114.240.*.*` instead of internal hash `anon_...`.
- Frontend UI: In `formatters.js`, strip wildcard prefixes (`*.`) when generating page preview URLs; in `analytics.js`, set `visitorViewMode` before calling `setProjectTab` to avoid redundant stream network requests and timer churn.

**Tech Stack:** Go 1.25, PostgreSQL (pgx/v5), Redis, Alpine.js, Tailwind/UnoCSS.

**Spec:** Current codebase review and confirmed bug report.

## Global Constraints
- Strict TDD: Write failing test first, verify failure, write minimal fix, verify pass.
- Minimal diffs, scoped directly to bug locations.
- Zero extraneous dependencies.
- Verify full test suite (`go test -count=1 ./...`) and race detector (`go test -race ./...`).

---

### Task 1: Fix `TodayOverview` Yesterday Same Period UV & IP Scale (BUG 1)

**Files:**
- Modify: `internal/metrics/today_overview.go:88-144`
- Test: `internal/metrics/today_overview_test.go`

**Interfaces:**
- `(s *Service) GetTodayOverview(ctx context.Context, projectID string, now time.Time) (*TodayOverview, error)`
  - `YesterdaySamePeriod.UV`: Scaled based on yesterday's hourly cumulative ratio multiplied by `yesterdayFull.UV`, capped at `yesterdayFull.UV`.
  - `YesterdaySamePeriod.IP`: Scaled based on yesterday's hourly cumulative ratio multiplied by `yesterdayFull.IP`, capped at `yesterdayFull.IP`.
  - At `currentHour >= 23`: `YesterdaySamePeriod.UV == yesterdayFull.UV` and `YesterdaySamePeriod.IP == yesterdayFull.IP`.

- [x] **Step 1: Write the failing test**
In `internal/metrics/today_overview_test.go`, add `TestTodayOverview_YesterdaySamePeriodUVAndIPScale`:
```go
func TestTodayOverview_YesterdaySamePeriodUVAndIPScale(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) // noon
	projectID := "test-proj-scale"

	// Mock live stats: today has 50 unique visitors, 40 IPs so far
	mockLive := &mockLiveStatsReader{
		todayStats: &analytics.DailyStats{
			ProjectID: projectID,
			Date:      "2026-09-29",
			PV:        200,
			UV:        50,
			IP:        40,
			Hourly: map[string]map[string]int64{
				"pv": {"10": 100, "11": 100},
				"uv": {"10": 25, "11": 25},
				"ip": {"10": 20, "11": 20},
			},
		},
	}

	// Mock DB with yesterday's stats:
	// yesterdayFull: PV=400, UV=100, IP=80
	// yesterday hourly UV sums to 200 (due to repeat visits across hours)
	// up to hour 12, yesterday hourly UV sum is 100 (50% of the day's activity)
	hourlyData := map[string]map[string]int64{
		"pv": {"10": 100, "11": 100, "14": 100, "15": 100},
		"uv": {"10": 50, "11": 50, "14": 50, "15": 50},
		"ip": {"10": 40, "11": 40, "14": 40, "15": 40},
	}
	hourlyJSON, _ := json.Marshal(hourlyData)

	mockPool := &mockDBQuerier{
		rows: []map[string]interface{}{
			{
				"pv":     int64(400),
				"uv":     int64(100),
				"ip":     int64(80),
				"hourly": hourlyJSON,
			},
		},
	}

	service := newForTest(mockPool, mockLive)
	overview, err := service.GetTodayOverview(context.Background(), projectID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// YesterdaySamePeriod UV should be ~50% of yesterdayFull (100 * 100 / 200 = 50), NOT the raw hourly sum of 100
	if overview.YesterdaySamePeriod.UV != 50 {
		t.Errorf("expected YesterdaySamePeriod.UV = 50, got %d", overview.YesterdaySamePeriod.UV)
	}
	if overview.YesterdaySamePeriod.IP != 40 {
		t.Errorf("expected YesterdaySamePeriod.IP = 40, got %d", overview.YesterdaySamePeriod.IP)
	}
	// Changes for UV: today (50) vs yesterdaySamePeriod (50) should be 0%, not -50%
	if overview.Changes.UV == nil || *overview.Changes.UV != 0.0 {
		t.Errorf("expected Changes.UV = 0.0, got %v", overview.Changes.UV)
	}
}
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/metrics -run TestTodayOverview_YesterdaySamePeriodUVAndIPScale`
Expected: FAIL with `expected YesterdaySamePeriod.UV = 50, got 100`.

- [x] **Step 3: Implement scaled calculation in `today_overview.go`**
In `internal/metrics/today_overview.go`, replace lines 106-114:
```go
	var (
		totalYesterdayHourlyUV int64
		totalYesterdayHourlyIP int64
		sumYesterdaySamePeriodUV int64
		sumYesterdaySamePeriodIP int64
	)
	for h := 0; h < 24; h++ {
		key := fmt.Sprintf("%02d", h)
		pt := yesterdayHourly[key]
		totalYesterdayHourlyUV += pt.UV
		totalYesterdayHourlyIP += pt.IP
		if h <= currentHour {
			sumYesterdaySamePeriodUV += pt.UV
			sumYesterdaySamePeriodIP += pt.IP
		}
	}

	var yesterdaySamePeriod PeriodTotals
	for h := 0; h <= currentHour; h++ {
		key := fmt.Sprintf("%02d", h)
		yesterdaySamePeriod.PV += yesterdayHourly[key].PV
	}

	if currentHour >= 23 {
		yesterdaySamePeriod.UV = yesterdayFull.UV
		yesterdaySamePeriod.IP = yesterdayFull.IP
	} else {
		if totalYesterdayHourlyUV > 0 && yesterdayFull.UV > 0 {
			scaled := int64(float64(yesterdayFull.UV)*float64(sumYesterdaySamePeriodUV)/float64(totalYesterdayHourlyUV) + 0.5)
			if scaled > yesterdayFull.UV {
				scaled = yesterdayFull.UV
			}
			yesterdaySamePeriod.UV = scaled
		} else {
			yesterdaySamePeriod.UV = sumYesterdaySamePeriodUV
		}

		if totalYesterdayHourlyIP > 0 && yesterdayFull.IP > 0 {
			scaled := int64(float64(yesterdayFull.IP)*float64(sumYesterdaySamePeriodIP)/float64(totalYesterdayHourlyIP) + 0.5)
			if scaled > yesterdayFull.IP {
				scaled = yesterdayFull.IP
			}
			yesterdaySamePeriod.IP = scaled
		} else {
			yesterdaySamePeriod.IP = sumYesterdaySamePeriodIP
		}
	}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/metrics -run TestTodayOverview_`
Expected: PASS

---

### Task 2: Fix `trimBreakdowns` for `paths` in `internal/metrics/metrics.go` (BUG 2)

**Files:**
- Modify: `internal/metrics/metrics.go:813`
- Test: `internal/metrics/metrics_test.go`

**Interfaces:**
- `var breakdownLimits = map[string]int{"channels": 100, "paths": 100}`

- [x] **Step 1: Write the failing test**
In `internal/metrics/metrics_test.go`, add:
```go
func TestTrimBreakdowns_PathsRetainsUpTo100(t *testing.T) {
	breakdowns := map[string]map[string]int64{
		"paths": make(map[string]int64),
	}
	for i := 0; i < 50; i++ {
		breakdowns["paths"][fmt.Sprintf("/page-%d", i)] = int64(i + 1)
	}

	trimBreakdowns(breakdowns, 20)

	if len(breakdowns["paths"]) != 50 {
		t.Fatalf("expected 50 paths retained, got %d", len(breakdowns["paths"]))
	}
}
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/metrics -run TestTrimBreakdowns_PathsRetainsUpTo100`
Expected: FAIL (got 20).

- [x] **Step 3: Update `breakdownLimits` in `metrics.go`**
In `internal/metrics/metrics.go`:
```go
var breakdownLimits = map[string]int{"channels": 100, "paths": 100}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/metrics -run TestTrimBreakdowns_PathsRetainsUpTo100`
Expected: PASS

---

### Task 3: Fix `getPagePreviewUrl` Wildcard Domain Stripping (BUG 3)

**Files:**
- Modify: `web/static/js/modules/formatters.js:268-276`

**Interfaces:**
- `getPagePreviewUrl(path)`: If `domains[0]` starts with `*.`, strip `*.` before constructing `https://${domain}${cleanPath}`.

- [x] **Step 1: Update `getPagePreviewUrl` in `formatters.js`**
In `web/static/js/modules/formatters.js`:
```javascript
        getPagePreviewUrl(path) {
            if (!path) return '#';
            if (path.startsWith('http://') || path.startsWith('https://')) return path;
            const homepage = this.getHomepageLink();
            if (homepage) {
                const base = homepage.replace(/\/+$/, '');
                const cleanPath = path.startsWith('/') ? path : `/${path}`;
                return `${base}${cleanPath}`;
            }
            const domains = this.selectedProject?.websiteDomains || [];
            if (domains.length > 0 && domains[0]) {
                let domain = domains[0].trim();
                domain = domain.replace(/^\*\./, '');
                if (!domain.includes('://')) {
                    domain = `https://${domain}`;
                }
                const base = domain.replace(/\/+$/, '');
                const cleanPath = path.startsWith('/') ? path : `/${path}`;
                return `${base}${cleanPath}`;
            }
            return path.startsWith('/') ? path : `/${path}`;
        },
```

- [x] **Step 2: Verify in browser / node test**
Verify with node one-liner:
```bash
node -e "
const formatters = {
  selectedProject: { websiteDomains: ['*.example.com'] },
  getHomepageLink: () => '',
  getPagePreviewUrl(path) {
    let domain = this.selectedProject.websiteDomains[0].replace(/^\*\./, '');
    return 'https://' + domain + (path.startsWith('/') ? path : '/' + path);
  }
};
console.assert(formatters.getPagePreviewUrl('/test') === 'https://example.com/test');
"
```

---

### Task 4: Fix `TrackBadgeOrCounter` Scheme-less Referrer Path Parsing (BUG 4)

**Files:**
- Modify: `internal/analytics/analytics.go:265-274`
- Test: `internal/api/svg_tracking_test.go`

**Interfaces:**
- `TrackBadgeOrCounter`: If `referrer` has no scheme, prepend `https://` before calling `url.Parse(referrer)` so hostnames like `"github.com"` are not parsed into `path = "github.com"`.

- [x] **Step 1: Write the failing test**
In `internal/api/svg_tracking_test.go`, add `TestTrackBadgeOrCounter_SchemelessReferrerPath`:
```go
func TestTrackBadgeOrCounter_SchemelessReferrerPath(t *testing.T) {
	proj := &project.Project{
		ID:            "schemeless-ref-project",
		Slug:          "schemeless-ref-slug",
		Status:        "active",
		RenderEnabled: true,
	}
	_, analyticsSvc, projectID := newSVGTestAPIApp(t, proj)
	ctx := context.Background()

	req := httptest.NewRequest("GET", "http://example.com/svg/schemeless-ref-slug/counter/views.svg", nil)
	req.Header.Set("Referer", "github.com") // scheme-less referrer
	req.RemoteAddr = "120.24.1.1:1234"

	if err := analyticsSvc.TrackBadgeOrCounter(ctx, req, projectID); err != nil {
		t.Fatalf("TrackBadgeOrCounter failed: %v", err)
	}

	stats, err := analyticsSvc.GetTodayStats(ctx, projectID)
	if err != nil {
		t.Fatalf("GetTodayStats failed: %v", err)
	}

	// Should be recorded under "/" not "github.com"
	if stats.Paths["github.com"] > 0 {
		t.Errorf("path 'github.com' was recorded; expected root path '/'")
	}
	if stats.Paths["/"] != 1 {
		t.Errorf("expected path '/' count = 1, got %v", stats.Paths)
	}
}
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/api -run TestTrackBadgeOrCounter_SchemelessReferrerPath`
Expected: FAIL (recorded `github.com` instead of `/`).

- [x] **Step 3: Normalize scheme in `TrackBadgeOrCounter`**
In `internal/analytics/analytics.go`:
```go
	path := ""
	if referrer != "" {
		refToParse := strings.TrimSpace(referrer)
		if !strings.Contains(refToParse, "://") {
			refToParse = "https://" + refToParse
		}
		if refURL, err := url.Parse(refToParse); err == nil {
			if refURL.Path != "" {
				path = refURL.Path
			} else if refURL.Hostname() != "" {
				path = "/"
			}
		}
	}
```

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api -run TestTrackBadgeOrCounter_`
Expected: PASS

---

### Task 5: Store `MaskedIP` in VisitorDetail & Update UI Table (BUG 5)

**Files:**
- Modify: `internal/analytics/analytics.go:574, 1446-1463`
- Modify: `web/components/project/TabVisitors.html:241`

**Interfaces:**
- `VisitorDetail.MaskedIP`: Exposed as `maskedIp` JSON field with `maskIP(rawIP)`.
- `TabVisitors.html`: Display `item.maskedIp || item.ip || '—'`.

- [x] **Step 1: Write failing test for MaskedIP in VisitorDetail**
In `internal/analytics/analytics_test.go` or `internal/analytics/ip_hourly_test.go`:
```go
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
```

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/analytics -run TestGetTodayVisitors_MaskedIPPreserved`
Expected: FAIL.

- [x] **Step 3: Save `masked_ip` in `visitorKey` and parse in `buildVisitorDetail`**
In `internal/analytics/analytics.go`:
1. Add `MaskedIP string `json:"maskedIp,omitempty"`` to `VisitorDetail`.
2. In `trackRequestData`:
   `"masked_ip": maskIP(data.RawIP),`
3. In `buildVisitorDetail`:
   `detail.MaskedIP = data["masked_ip"]`
   If `detail.MaskedIP == ""` and `data["ip"] != ""` && !strings.HasPrefix(data["ip"], "anon_") {
       detail.MaskedIP = maskIP(data["ip"])
   }
4. In `web/components/project/TabVisitors.html:241`:
   `<div class="font-mono text-xs font-semibold text-gray-800 tracking-tight" x-text="item.maskedIp || item.ip || '—'"></div>`

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/analytics -run TestGetTodayVisitors_MaskedIPPreserved`
Expected: PASS

---

### Task 6: Add PostgreSQL Migration and Worker Persistence for `ai_visits` (BUG 6)

**Files:**
- Create: `migrations/026_add_daily_statistics_ai_visits.sql`
- Modify: `internal/worker/worker.go:242, 248, 464`
- Modify: `internal/worker/worker_test.go`

**Interfaces:**
- Column `ai_visits BIGINT NOT NULL DEFAULT 0` on `daily_statistics`.
- `upsertDailyStats` in `worker.go` persists `ai_visits`.

- [x] **Step 1: Create migration file**
Create `migrations/026_add_daily_statistics_ai_visits.sql`:
```sql
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS ai_visits BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN daily_statistics.ai_visits IS 'Daily AI crawler and search agent visit count';
```

- [x] **Step 2: Update `worker.go` query and args**
In `internal/worker/worker.go`:
1. Add `ai_visits` to columns in `upsertDailyStats`:
   `terms, channels, site_searches, ip, hourly, ai_visits`
   VALUES: `$1..$42`
   ON CONFLICT DO UPDATE SET `ai_visits = EXCLUDED.ai_visits`
2. Add `stats.AIVisits` to returned args in `dailyStatsArgs`.

- [x] **Step 3: Update `worker_test.go`**
Update expected args count in `worker_test.go`.

- [x] **Step 4: Run worker tests**
Run: `go test -v ./internal/worker/...`
Expected: PASS

---

### Task 7: Fix `navigateToIpVisitors` and `navigateToStreamVisitors` Sequence (BUG 7)

**Files:**
- Modify: `web/static/js/modules/analytics.js:190-198`

**Interfaces:**
- `navigateToIpVisitors`: Set `this.switchVisitorViewMode('ip')` BEFORE `this.setProjectTab('visitors')`.
- `navigateToStreamVisitors`: Set `this.switchVisitorViewMode('stream')` BEFORE `this.setProjectTab('visitors')`.

- [x] **Step 1: Update `analytics.js`**
In `web/static/js/modules/analytics.js`:
```javascript
        navigateToStreamVisitors() {
            this.switchVisitorViewMode('stream');
            this.setProjectTab('visitors');
        },

        navigateToIpVisitors() {
            this.switchVisitorViewMode('ip');
            this.setProjectTab('visitors');
        },
```

---

### Task 8: Full Regression Suite Verification

- [x] Run `go test -count=1 ./...`
- [x] Run `go test -race ./...`
- [x] Run `go vet ./...`
- [x] Run `go build ./cmd/...`
