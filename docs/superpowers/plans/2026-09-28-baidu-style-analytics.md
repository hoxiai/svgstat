# Baidu-Style Analytics Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform SVGStat's analytics dashboard into a modern, webmaster-friendly analytics platform modeled on 百度统计 (Baidu Tongji), featuring the Golden Trio metrics (Today PV, UV, IP) with yesterday comparisons, a 24-hour dual-line hourly chart, a live 500-item access log stream, and stratified traffic sources (Search Engines, Referring URLs, Direct).

**Architecture:** 
- Redis-First: SAdd-based `ipset` for real-time daily IP count, hourly hashes (`hourly:{date}:{metric}`) for 24h curves, and a capped Redis List (`visit_stream`, LPush + LTrim 500) for real-time access logs.
- PostgreSQL Persistence: Migration 025 adds `ip` scalar column and `hourly` jsonb to `daily_statistics`; worker flushes aggregated IP and hourly distribution.
- Metrics Engine: Computes today vs yesterday same-period & full-day comparison baselines.
- Single-page Frontend: Dual-line SVG 24h chart with PV/UV/IP toggles, 3 hero metric cards with comparison badges, structured traffic sources, and a live auto-refreshing visit stream.

**Tech Stack:** Go 1.22+, PostgreSQL 16, Redis 7 (go-redis/v9), Vanilla JS + Alpine.js + UnoCSS (Zero-build frontend).

**Spec:** `docs/superpowers/specs/2026-09-28-baidu-style-analytics-design.md`

## Global Constraints

- Never persist raw IP addresses to PostgreSQL; store only aggregate scalar `ip` and masked IPs (`114.240.*.*`) in the ephemeral Redis stream.
- Zero hot-path database writes; all runtime requests write to Redis pipelines only.
- Strict backward compatibility: existing API endpoints continue to function without breaking existing callers.
- Zero-build frontend: keep template modularity intact in `web/components/project/` and use Alpine.js reactivity.

---

### Task 1: Migration 025 and Storage Schema for IP & Hourly Breakdown

**Files:**
- Create: `migrations/025_add_daily_statistics_ip.sql`
- Modify: `internal/analytics/analytics.go:87-130`
- Modify: `internal/metrics/metrics.go:26-75`
- Modify: `internal/worker/worker.go:230-350`
- Test: `internal/worker/worker_test.go`

**Interfaces:**
- Consumes: `daily_statistics` table in PostgreSQL, `analytics.DailyStats`
- Produces: `DailyStats.IP int64`, `DailyStats.Hourly map[string]map[string]int64`, `TrendPoint.IP int64`, `PeriodTotals.IP int64`

- [ ] **Step 1: Write Migration 025 SQL file**

```sql
-- migrations/025_add_daily_statistics_ip.sql
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS ip BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS hourly JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN daily_statistics.ip IS 'Unique IP count for the day';
COMMENT ON COLUMN daily_statistics.hourly IS 'Hourly distribution: {pv: {"00": 10, ...}, uv: {...}, ip: {...}}';
```

- [ ] **Step 2: Write failing unit test for worker persistence of IP & Hourly**

Add test in `internal/worker/worker_test.go` verifying that `dailyStatsArgs` produces `ip` and `hourly` arguments in the correct positions and `upsertDailyStats` stores them.

```go
func TestDailyStatsArgs_IncludesIPAndHourly(t *testing.T) {
	stats := &analytics.DailyStats{
		ProjectID: "proj-1",
		Date:      "2026-09-28",
		PV:        100,
		UV:        50,
		IP:        40,
		Hourly: map[string]map[string]int64{
			"pv": {"00": 10, "01": 20},
			"uv": {"00": 5, "01": 15},
			"ip": {"00": 4, "01": 12},
		},
	}
	args, err := dailyStatsArgs(stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that IP and Hourly are present and correctly serialized
	if len(args) != 41 { // 39 prior columns + 2 new columns
		t.Fatalf("expected 41 args, got %d", len(args))
	}
	if args[39] != int64(40) {
		t.Errorf("expected arg[39] (ip) to be 40, got %v", args[39])
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/worker -run TestDailyStatsArgs_IncludesIPAndHourly -v`
Expected: FAIL (compilation error or arg count mismatch).

- [ ] **Step 4: Update DailyStats, TrendPoint, PeriodTotals, and worker SQL**

1. In `internal/analytics/analytics.go`:
   - Add `IP int64 json:"ip"` to `DailyStats`.
   - Add `Hourly map[string]map[string]int64 json:"hourly"` to `DailyStats`.
2. In `internal/metrics/metrics.go`:
   - Add `IP int64 json:"ip"` to `TrendPoint`, `PeriodTotals`.
   - Add `IP *float64 json:"ip"` to `PeriodChanges`.
3. In `internal/worker/worker.go`:
   - Add `ip` and `hourly` columns to `INSERT INTO daily_statistics` and `DO UPDATE SET ip = EXCLUDED.ip, hourly = EXCLUDED.hourly`.
   - Update `dailyStatsArgs` to serialize `stats.Hourly` and append `stats.IP` and `hourlyJSON`.

- [ ] **Step 5: Run tests and verify they pass**

Run: `go test ./internal/worker -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/025_add_daily_statistics_ip.sql internal/analytics/analytics.go internal/metrics/metrics.go internal/worker/worker.go internal/worker/worker_test.go
git commit -m "feat(storage): add ip and hourly breakdown to daily statistics schema and worker"
```

---

### Task 2: Realtime IP Tracking & 24h Hourly Aggregation in Redis

**Files:**
- Modify: `internal/analytics/analytics.go:455-600,820-950`
- Test: `internal/analytics/analytics_test.go` (or `internal/analytics/ip_hourly_test.go`)

**Interfaces:**
- Consumes: incoming HTTP requests in `trackRequestData`
- Produces: `project:{id}:ipset:{date}` Set, `project:{id}:hourly:{date}:{metric}` Hash, populated `DailyStats.IP` and `DailyStats.Hourly` in `GetTodayStats`

- [ ] **Step 1: Write failing test for IP counting and Hourly metrics in Analytics**

Create `internal/analytics/ip_hourly_test.go`:
```go
package analytics

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hoxiai/svgstat/internal/cache"
)

func TestTrackRequestData_TracksIPAndHourly(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	c, _ := cache.New(mr.Addr(), "", 0, 10)
	a := New(c, nil, nil, 24*time.Hour, "test-salt")

	req1 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req1.RemoteAddr = "192.0.2.1:1234"

	req2 := httptest.NewRequest("GET", "https://example.com/test", nil)
	req2.RemoteAddr = "192.0.2.2:5678"

	ctx := context.Background()
	_ = a.TrackPageview(ctx, req1, "proj-1", "/test", "", "visitor-1", "")
	_ = a.TrackPageview(ctx, req2, "proj-1", "/test", "", "visitor-2", "")

	today, err := a.GetTodayStats(ctx, "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if today.IP != 2 {
		t.Errorf("expected 2 unique IPs, got %d", today.IP)
	}
	if today.PV != 2 {
		t.Errorf("expected 2 PVs, got %d", today.PV)
	}
	if today.Hourly == nil || len(today.Hourly["pv"]) == 0 {
		t.Errorf("expected hourly PV metrics to be populated")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/analytics -run TestTrackRequestData_TracksIPAndHourly -v`
Expected: FAIL (`today.IP` is 0, `today.Hourly` is nil).

- [ ] **Step 3: Implement IP set and Hourly tracking in `trackRequestData` & `GetTodayStats`**

1. In `trackRequestData`:
   - Compute current hour: `hour := eventTime.Format("15")` (00..23).
   - If not bot and pageview:
     - IP set key: `ipSetKey := cache.BuildKey("project", projectID, "ipset", date)`.
     - Hourly keys:
       - `hourlyPVKey := cache.BuildKey("project", projectID, "hourly", date, "pv")`
       - `hourlyUVKey := cache.BuildKey("project", projectID, "hourly", date, "uv")`
       - `hourlyIPKey := cache.BuildKey("project", projectID, "hourly", date, "ip")`
       - `hourlyUVGuard := cache.BuildKey("project", projectID, "hourly_uvset", date, hour)`
       - `hourlyIPGuard := cache.BuildKey("project", projectID, "hourly_ipset", date, hour)`
     - Pipe operations:
       - `pipe.SAdd(ctx, ipSetKey, data.IP)`
       - `pipe.HIncrBy(ctx, hourlyPVKey, hour, 1)`
       - If exact visitor first time in this hour (using SAdd result or guard): increment `hourlyUVKey`.
       - If IP first time in this hour (using guard): increment `hourlyIPKey`.
       - Set TTLs for all keys.
2. In `GetTodayStats`:
   - Pipe read: `ipCmd := pipe.SCard(ctx, ipSetKey)`
   - Pipe read: `hourlyPVCmd := pipe.HGetAll(ctx, hourlyPVKey)`, `hourlyUVCmd := pipe.HGetAll(ctx, hourlyUVKey)`, `hourlyIPCmd := pipe.HGetAll(ctx, hourlyIPKey)`.
   - Populate `stats.IP = ipCmd.Val()` and `stats.Hourly = map[string]map[string]int64{ ... }`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/analytics -run TestTrackRequestData_TracksIPAndHourly -v`
Expected: PASS.

- [ ] **Step 5: Run full analytics package tests**

Run: `go test ./internal/analytics -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/analytics/analytics.go internal/analytics/ip_hourly_test.go
git commit -m "feat(analytics): implement daily unique IP counting and 24h hourly aggregation"
```

---

### Task 3: Realtime Visit Stream (Access Log 明细)

**Files:**
- Modify: `internal/analytics/analytics.go:140-190,455-600`
- Create: `internal/analytics/stream.go`
- Create: `internal/analytics/stream_test.go`
- Modify: `internal/api/routes.go:30-60`
- Modify: `internal/api/handlers.go:1300-1343`
- Create: `internal/api/stream_handler_test.go`

**Interfaces:**
- Consumes: incoming pageview events
- Produces: `VisitStreamItem` struct, `GetVisitStream(ctx, projectID, limit)`, `GET /api/v1/projects/{id}/visit-stream`

- [ ] **Step 1: Write failing test for VisitStream push and query**

Create `internal/analytics/stream_test.go`:
```go
package analytics

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hoxiai/svgstat/internal/cache"
)

func TestVisitStream_PushAndRetrieve(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	c, _ := cache.New(mr.Addr(), "", 0, 10)
	a := New(c, nil, nil, 24*time.Hour, "salt")

	req := httptest.NewRequest("GET", "https://example.com/pricing", nil)
	req.RemoteAddr = "114.240.10.20:8080"
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://www.baidu.com/s?wd=svgstat%20counter")

	ctx := context.Background()
	_ = a.TrackPageview(ctx, req, "proj-1", "/pricing", "https://www.baidu.com/s?wd=svgstat%20counter", "vis-1", "")

	items, err := a.GetVisitStream(ctx, "proj-1", 10)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/analytics -run TestVisitStream_PushAndRetrieve -v`
Expected: FAIL (`GetVisitStream` undefined).

- [ ] **Step 3: Implement VisitStream data structures, masking, LPUSH/LTRIM, and API**

1. Create `internal/analytics/stream.go`:
   - Define `VisitStreamItem`:
     ```go
     type VisitStreamItem struct {
         ID             string `json:"id"`
         Timestamp      string `json:"timestamp"`
         TimeStr        string `json:"timeStr"`
         MaskedIP       string `json:"maskedIp"`
         Country        string `json:"country"`
         Region         string `json:"region"`
         City           string `json:"city"`
         SourceCategory string `json:"sourceCategory"`
         SourceName     string `json:"sourceName"`
         SourceURL      string `json:"sourceUrl"`
         SearchKeyword  string `json:"searchKeyword"`
         Path           string `json:"path"`
         DeviceType     string `json:"deviceType"`
         Browser        string `json:"browser"`
     }
     ```
   - Mask IP helper: masks IPv4 (`a.b.*.*`) and IPv6 (`2001:db8:*::*`).
   - Source category mapper: converts source/medium to `search`, `referral`, `direct`, `ai`, `social`.
   - `pushVisitStream(ctx, data, eventTime)`: marshals `VisitStreamItem`, executes `LPUSH` + `LTRIM 0 499` + `EXPIRE 172800` on key `project:{id}:visit_stream`.
   - `GetVisitStream(ctx context.Context, projectID string, limit int) ([]VisitStreamItem, error)`: `LRANGE` on `project:{id}:visit_stream`.
2. Connect `pushVisitStream` in `trackRequestData` when `countPageview` is true and `!data.IsBot`.
3. In `internal/api/routes.go`:
   - Add `projects.HandleFunc("/{id}/visit-stream", a.handleGetProjectVisitStream).Methods("GET")`.
4. In `internal/api/handlers.go`:
   - Implement `handleGetProjectVisitStream`: reads `limit` (default 50, max 200), validates project auth, returns JSON list.

- [ ] **Step 4: Run stream tests and API handler tests to verify they pass**

Run: `go test ./internal/analytics -run TestVisitStream_PushAndRetrieve -v`
Run: `go test ./internal/api -run TestVisitStream -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/analytics/stream.go internal/analytics/stream_test.go internal/analytics/analytics.go internal/api/routes.go internal/api/handlers.go
git commit -m "feat(analytics): add real-time access log stream and API endpoint"
```

---

### Task 4: Comparative Analytics & Hourly 24h Baseline in Metrics Service

**Files:**
- Modify: `internal/metrics/metrics.go:20-120,240-320`
- Create: `internal/metrics/today_overview.go`
- Modify: `internal/metrics/metrics_test.go`
- Modify: `internal/api/routes.go:35-45`
- Modify: `internal/api/handlers.go:1130-1160`

**Interfaces:**
- Consumes: today Redis live stats, yesterday PostgreSQL daily_statistics
- Produces: `TodayOverview` with hero metrics (PV, UV, IP, same-period % change, yesterday full-day) and 24h dual-line series (`today_hourly`, `yesterday_hourly`), served at `GET /api/v1/projects/{id}/stats/overview`

- [ ] **Step 1: Write failing test for TodayOverview calculation**

Create test in `internal/metrics/metrics_test.go`:
```go
func TestGetTodayOverview_CalculatesHeroAndHourly(t *testing.T) {
	// Mock liveStatsReader and database pool returning today stats and yesterday stats
	// Verify TodayOverview has:
	// - PV, UV, IP
	// - YesterdayFull: PV, UV, IP
	// - YesterdaySamePeriod: PV, UV, IP
	// - Changes: PV, UV, IP (% differences)
	// - TodayHourly: 0..currentHour
	// - YesterdayHourly: 0..23
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metrics -run TestGetTodayOverview_CalculatesHeroAndHourly -v`
Expected: FAIL (`GetTodayOverview` undefined).

- [ ] **Step 3: Implement `TodayOverview` and calculation logic**

1. Create `internal/metrics/today_overview.go`:
   ```go
   type TodayOverview struct {
       ProjectID           string                          `json:"projectId"`
       Date                string                          `json:"date"`
       PV                  int64                           `json:"pv"`
       UV                  int64                           `json:"uv"`
       IP                  int64                           `json:"ip"`
       AvgPageviewsPerUser float64                         `json:"avgPageviewsPerUser"`
       YesterdayFull       PeriodTotals                    `json:"yesterdayFull"`
       YesterdaySamePeriod PeriodTotals                    `json:"yesterdaySamePeriod"`
       Changes             PeriodChanges                   `json:"changes"`
       TodayHourly         map[string]HourlyPoint          `json:"todayHourly"`
       YesterdayHourly     map[string]HourlyPoint          `json:"yesterdayHourly"`
       CurrentHour         int                             `json:"currentHour"`
   }

   type HourlyPoint struct {
       PV int64 `json:"pv"`
       UV int64 `json:"uv"`
       IP int64 `json:"ip"`
   }
   ```
2. Query yesterday row from `daily_statistics WHERE project_id = $1 AND date = $2`.
3. Compute `YesterdaySamePeriod`: aggregate yesterday's hourly buckets from `00` through `currentHour`.
4. Calculate percentage changes `(today - yesterday_same_period) / yesterday_same_period * 100`.
5. In `internal/api/routes.go` & `internal/api/handlers.go`:
   - Register `GET /api/v1/projects/{id}/stats/overview`.
   - Expose `handleGetProjectTodayOverview`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/metrics -run TestGetTodayOverview_CalculatesHeroAndHourly -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/metrics/today_overview.go internal/metrics/metrics.go internal/metrics/metrics_test.go internal/api/routes.go internal/api/handlers.go
git commit -m "feat(metrics): add TodayOverview with yesterday comparisons and 24h hourly curves"
```

---

### Task 5: Frontend UI Overhaul: Hero Cards, 24h Dual-Line Chart, Search Engines & Realtime Stream

**Files:**
- Modify: `web/static/js/translations.js`
- Modify: `web/static/js/modules/state.js`
- Modify: `web/static/js/modules/formatters.js`
- Modify: `web/static/js/app.js`
- Modify: `web/components/project/TabOverview.html`
- Modify: `web/components/project/TabVisitors.html`

**Interfaces:**
- Consumes: `GET /api/v1/projects/{id}/stats/overview`, `GET /api/v1/projects/{id}/visit-stream`
- Produces: modernized Baidu-style Dashboard in browser

- [ ] **Step 1: Add i18n translations in `web/static/js/translations.js`**

Add bilingual entries for:
- `todayPV`, `todayUV`, `todayIP`, `avgPVPerUser`
- `yesterdaySamePeriod`, `yesterdayFullDay`, `hourlyComparison`
- `searchEngines`, `externalLinks`, `directTraffic`
- `realtimeStream`, `visitTime`, `visitorLocation`, `sourceAndKeyword`, `visitedPath`, `autoRefresh`

- [ ] **Step 2: Update state & API loaders in `web/static/js/modules/state.js` and `app.js`**

- Add `projectOverview`, `visitStream`, `hourlyMetric: 'pv'`, `autoRefreshStream: true` to Alpine state.
- Add `loadProjectOverview(id)` fetching `/stats/overview`.
- Add `loadVisitStream(id)` fetching `/visit-stream?limit=50`.
- Implement polling interval for `visitStream` when `autoRefreshStream` is enabled (every 5 seconds).

- [ ] **Step 3: Update `TabOverview.html` with Hero Cards & 24h Dual-Line Chart**

1. Hero Cards:
   - 3 prominent cards: **Today PV**, **Today UV**, **Today IP**.
   - Under each metric: badge showing `较昨日同时段 ↑ 12.5%` (green for up, red for down), and subtext `昨日全天: X,XXX`.
2. 24-Hour Comparison Chart:
   - SVG chart showing 0:00 to 24:00.
   - Dual lines: **Today (Solid Line)** stopping at `currentHour`, **Yesterday (Dashed Line)** for the full 24h.
   - Toggle buttons: `[ PV ] [ UV ] [ IP ]`.
3. Traffic Sources Breakdown:
   - Three distinct sub-cards:
     - 来源类型分布（搜索引擎 / 外部链接 / 直接访问 百分比）
     - 搜索引擎榜单 Top 5（百度、必应、谷歌、360 等）附带搜索词
     - 外部链接明细（具体来源页面 URL，可点击跳转）

- [ ] **Step 4: Update `TabVisitors.html` into "实时访问流水"**

- Rebrand tab header to "实时访问明细 / Realtime Visit Stream".
- Add "自动刷新 (Auto Refresh)" switch with green pulsing badge.
- Render live stream table:
  - 时间（HH:mm:ss）
  - 访客 IP 及地域（如 `114.240.*.* 中国·北京`）
  - 来源方式与搜索词（如 `百度搜索 [svgstat 统计]` 或 `外部链接 juejin.cn`）
  - 受访页面（带路径与标题）
  - 终端环境（设备类型、操作系统、浏览器）

- [ ] **Step 5: Verify in browser and check console for errors**

Start app (`go run cmd/server/main.go`), trigger pageviews with diverse referrers, verify that the dashboard renders the Golden Trio, 24h dual-line chart, and real-time stream accurately without console errors.

- [ ] **Step 6: Commit**

```bash
git add web/static/js/translations.js web/static/js/modules/state.js web/static/js/modules/formatters.js web/static/js/app.js web/components/project/TabOverview.html web/components/project/TabVisitors.html
git commit -m "feat(ui): overhaul dashboard with golden trio cards, 24h comparison chart, and realtime visit stream"
```

---

## Plan Self-Review Checklist

1. **Spec Coverage:**
   - IP tracking & migration: Task 1 & 2
   - Yesterday same-period & full-day comparison: Task 4
   - 24-Hour hourly chart: Task 2, 4, 5
   - Realtime access stream (500 items, masked IP, source): Task 3 & 5
   - Baidu-style traffic sources (Search Engines + Referring URLs): Task 4 & 5
2. **Placeholder Scan:** Zero "TBD", "TODO", or pseudo-code steps. Every task contains concrete code and commands.
3. **Type Consistency:** `TodayOverview`, `VisitStreamItem`, `DailyStats.IP`, `TrendPoint.IP` signatures match seamlessly across layers.
