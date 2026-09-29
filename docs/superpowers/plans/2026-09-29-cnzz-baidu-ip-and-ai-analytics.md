# CNZZ & Baidu-Style Analytics Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a complete CNZZ and Baidu Tongji-style analytics experience in SVGStat featuring IP/PV/UV tracking, SVG request tracking, an aggregated IP visitor list, visited pages breakdown, AI agent & referral detection, and a flicker-free SWR UI.

**Architecture:**
- Hot-path tracking via Redis pipelines with zero database queries on incoming requests.
- SVG counter/badge handler integration with full pageview attribution (`countRequest: true, countPageview: true`).
- AI crawler and agent detection engine classifying autonomous spiders (GPTBot, ClaudeBot, PerplexityBot, Bytespider, DeepSeekBot, etc.) into live streams and AI metrics.
- Dual-view Visitors tab switching between realtime stream and today's IP aggregated list.
- Dedicated visited pages breakdown table in the Overview tab with path searching and share percentages.
- SWR silent background polling and incremental stream prepending to eliminate UI re-render flickering.

**Tech Stack:** Go 1.22+, Redis 7+, PostgreSQL, Alpine.js, UnoCSS.

**Spec:** `docs/superpowers/specs/2026-09-29-cnzz-baidu-ip-and-ai-analytics-design.md`

## Global Constraints

- Never persist raw IP addresses to PostgreSQL; store only aggregate scalar `ip` and masked IPs (`114.240.*.*`) in ephemeral Redis data structures.
- Zero hot-path database writes; all runtime requests write to Redis pipelines only.
- Strict backward compatibility: existing API endpoints continue to function without breaking existing callers.
- Zero-build frontend: keep template modularity intact in `web/components/project/` and use Alpine.js reactivity without node build steps.

---

### Task 1: AI Crawler Detection & Stream Extension

**Files:**
- Create: `internal/analytics/ai_detector.go`
- Create: `internal/analytics/ai_detector_test.go`
- Modify: `internal/analytics/stream.go:23-40,135-177`
- Modify: `internal/analytics/analytics.go:500-540`

**Interfaces:**
- Consumes: `RequestData.UserAgent`, `Analytics.cache`
- Produces:
  ```go
  func DetectAICrawler(userAgent string) (isAI bool, aiName string)
  ```
  `VisitStreamItem.IsAIAgent bool`, `VisitStreamItem.AIName string`

- [ ] **Step 1: Write the failing test**
Create `internal/analytics/ai_detector_test.go`:
```go
package analytics

import "testing"

func TestDetectAICrawler(t *testing.T) {
	cases := []struct {
		ua       string
		wantAI   bool
		wantName string
	}{
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)", true, "OpenAI (GPTBot)"},
		{"Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)", true, "Anthropic (ClaudeBot)"},
		{"Mozilla/5.0 (compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)", true, "Perplexity"},
		{"Mozilla/5.0 (Linux; Android 5.0) AppleWebKit/537.36 (KHTML, like Gecko) Mobile Safari/537.36 (compatible; Bytespider; spider-feedback@bytedance.com)", true, "字节跳动 (豆包/Bytespider)"},
		{"Mozilla/5.0 (compatible; DeepSeekBot/1.0; +https://www.deepseek.com)", true, "DeepSeek (深度求索)"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36", false, ""},
		{"curl/7.68.0", false, ""},
	}

	for _, tc := range cases {
		isAI, name := DetectAICrawler(tc.ua)
		if isAI != tc.wantAI || name != tc.wantName {
			t.Errorf("DetectAICrawler(%q) = (%v, %q), want (%v, %q)", tc.ua, isAI, name, tc.wantAI, tc.wantName)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/analytics/ -run TestDetectAICrawler`
Expected: FAIL (undefined `DetectAICrawler`)

- [ ] **Step 3: Implement minimal code**
Create `internal/analytics/ai_detector.go`:
```go
package analytics

import "strings"

type aiRule struct {
	keyword string
	name    string
}

var aiRules = []aiRule{
	{keyword: "gptbot", name: "OpenAI (GPTBot)"},
	{keyword: "chatgpt-user", name: "OpenAI (ChatGPT-User)"},
	{keyword: "oai-searchbot", name: "OpenAI (SearchBot)"},
	{keyword: "claudebot", name: "Anthropic (ClaudeBot)"},
	{keyword: "claude-web", name: "Anthropic (ClaudeWeb)"},
	{keyword: "anthropic-ai", name: "Anthropic (Claude)"},
	{keyword: "perplexitybot", name: "Perplexity"},
	{keyword: "bytespider", name: "字节跳动 (豆包/Bytespider)"},
	{keyword: "deepseekbot", name: "DeepSeek (深度求索)"},
	{keyword: "google-extended", name: "Google (Gemini)"},
	{keyword: "applebot-extended", name: "Apple (Applebot)"},
	{keyword: "meta-externalagent", name: "Meta (Llama)"},
	{keyword: "cohere-ai", name: "Cohere"},
	{keyword: "moonshot", name: "Moonshot (Kimi)"},
	{keyword: "kimibot", name: "Moonshot (Kimi)"},
	{keyword: "youbot", name: "You.com"},
}

// DetectAICrawler checks if a User-Agent belongs to an AI search agent or crawler.
func DetectAICrawler(userAgent string) (bool, string) {
	lower := strings.ToLower(userAgent)
	for _, rule := range aiRules {
		if strings.Contains(lower, rule.keyword) {
			return true, rule.name
		}
	}
	return false, ""
}
```

Update `VisitStreamItem` in `internal/analytics/stream.go`:
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
	IsAIAgent      bool   `json:"isAiAgent"`
	AIName         string `json:"aiName,omitempty"`
}
```

In `buildVisitStreamItem`:
```go
isAI, aiName := DetectAICrawler(data.UserAgent)
if isAI {
	item.IsAIAgent = true
	item.AIName = aiName
	item.SourceCategory = "ai"
	item.SourceName = aiName
	item.DeviceType = "bot"
	item.Browser = aiName
}
```

In `trackRequestData` in `internal/analytics/analytics.go`:
If `isAI, _ := DetectAICrawler(data.UserAgent); isAI`:
Increment `project:{id}:ai_visits:{date}` key and include in stream.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/analytics/ -run TestDetectAICrawler`
Expected: PASS

- [ ] **Step 5: Run full analytics test suite**
Run: `go test -v ./internal/analytics/...`
Expected: ALL PASS

- [ ] **Step 6: Commit**
```bash
git add internal/analytics/ai_detector.go internal/analytics/ai_detector_test.go internal/analytics/stream.go internal/analytics/analytics.go
git commit -m "feat(analytics): add AI crawler detection and stream attribution"
```

---

### Task 2: SVG Counter & Badge Full Pageview Tracking

**Files:**
- Modify: `internal/analytics/analytics.go:240-265`
- Modify: `internal/api/handlers.go:1040-1070,1110-1135`
- Test: `internal/api/svg_tracking_test.go`

**Interfaces:**
- Consumes: `*http.Request`, `projectID`
- Produces:
  ```go
  func (a *Analytics) TrackBadgeOrCounter(ctx context.Context, req *http.Request, projectID string) error
  ```

- [ ] **Step 1: Write the failing test**
Create `internal/api/svg_tracking_test.go`:
```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoxiai/svgstat/internal/cache"
)

func TestSVGTracking_RecordsPageviewAndStream(t *testing.T) {
	// Verify that TrackBadgeOrCounter attributes path from Referer and increments PV, UV, IP, and visit stream
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/api/ -run TestSVGTracking_RecordsPageviewAndStream`
Expected: FAIL (undefined `TrackBadgeOrCounter`)

- [ ] **Step 3: Implement minimal code**
In `internal/analytics/analytics.go`:
```go
// TrackBadgeOrCounter records an SVG badge or counter request as a full pageview and request.
func (a *Analytics) TrackBadgeOrCounter(ctx context.Context, req *http.Request, projectID string) error {
	data := a.extractRequestData(req, projectID)
	referrer := req.Referer()
	path := "/"
	if referrer != "" {
		if refURL, err := url.Parse(referrer); err == nil && refURL.Path != "" {
			path = refURL.Path
		}
	} else if req.URL != nil && req.URL.Path != "" {
		path = req.URL.Path
	}
	data.applyAttribution(websiteAttribution(path, referrer, originHost(req)))
	return a.trackRequestData(ctx, data, true, true)
}
```

In `internal/api/handlers.go`:
Replace `_ = a.analytics.TrackRequest(r.Context(), r, proj.ID)` in `handleCounterSVG` and `handleBadgeSVG` with:
```go
if !preview {
	_ = a.analytics.TrackBadgeOrCounter(r.Context(), r, proj.ID)
}
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/api/ -run TestSVGTracking_RecordsPageviewAndStream`
Expected: PASS

- [ ] **Step 5: Run full api test suite**
Run: `go test -v ./internal/api/...`
Expected: ALL PASS

- [ ] **Step 6: Commit**
```bash
git add internal/analytics/analytics.go internal/api/handlers.go internal/api/svg_tracking_test.go
git commit -m "feat(api): track SVG counter and badge requests as full pageviews"
```

---

### Task 3: AI Metric in Today Overview API

**Files:**
- Modify: `internal/metrics/today_overview.go:20-50,70-130`
- Test: `internal/metrics/today_overview_test.go`

**Interfaces:**
- Consumes: Redis key `project:{id}:ai_visits:{date}`
- Produces: `TodayOverview.TodayAI int64`

- [ ] **Step 1: Write the failing test**
Update `internal/metrics/today_overview_test.go` to assert `overview.TodayAI` matches recorded AI visits.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v ./internal/metrics/ -run TestTodayOverview`
Expected: FAIL

- [ ] **Step 3: Implement minimal code**
In `internal/metrics/today_overview.go`:
- Add `TodayAI int64 `json:"todayAi"`` to `TodayOverview`.
- In `GetTodayOverview`:
  - Read `aiKey := cache.BuildKey("project", projectID, "ai_visits", todayDate)`
  - Query Redis: `aiVisits, _ := s.cache.GetClient().Get(ctx, aiKey).Int64()`
  - Assign `overview.TodayAI = aiVisits`

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/metrics/ -run TestTodayOverview`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/metrics/today_overview.go internal/metrics/today_overview_test.go
git commit -m "feat(metrics): add today AI visits metric to TodayOverview"
```

---

### Task 4: Frontend Anti-Flicker SWR Refresh & Incremental Prepend

**Files:**
- Modify: `web/static/js/modules/analytics.js:60-120,460-520`
- Modify: `web/components/project/TabOverview.html:50-70`

**Interfaces:**
- Consumes: `loadVisitStream(projectId, isSilent)`, `loadProjectOverview(projectId, isSilent)`
- Produces: Flicker-free background updates without DOM tearing or clearing data

- [ ] **Step 1: Update `analytics.js` with silent refresh flag**
- In `loadProjectOverview(id, isSilent = false)`:
  - If `!isSilent && !this.projectOverview`, set `this.loadingOverview = true;`.
  - Fetch `/api/v1/projects/${id}/stats/overview`.
  - Assign `this.projectOverview = data.data;`.
- In `loadStats(id, isSilent = false)`:
  - If `!isSilent && !this.projectStats?.pv`, set `this.loadingStats = true;`.
  - Never call `this.projectStats = createEmptyProjectStats();` if data already exists.
- In `loadTrend(id, isSilent = false)`:
  - If `!isSilent && (!this.projectTrend || this.projectTrend.length === 0)`, set `this.loadingTrend = true;`.
- In `loadVisitStream(id, isSilent = false)`:
  - If `!isSilent && (!this.visitStream || this.visitStream.length === 0)`, set `this.loadingVisitStream = true;`.
  - Once data arrives:
    ```javascript
    const incoming = data.data || [];
    if (!this.visitStream || this.visitStream.length === 0) {
      this.visitStream = incoming;
    } else {
      const existingIds = new Set(this.visitStream.map(item => item.id));
      const newItems = incoming.filter(item => !existingIds.has(item.id));
      if (newItems.length > 0) {
        this.visitStream = [...newItems, ...this.visitStream].slice(0, 50);
      }
    }
    ```

- [ ] **Step 2: Update polling callers in `analytics.js`**
Ensure interval polling callbacks pass `isSilent = true`:
`this.loadVisitStream(id, true)` and `this.loadProjectOverview(id, true)`.

- [ ] **Step 3: Verify JS syntax**
Run: `node -c web/static/js/modules/analytics.js`
Expected: Clean exit (code 0)

- [ ] **Step 4: Commit**
```bash
git add web/static/js/modules/analytics.js web/components/project/TabOverview.html
git commit -m "fix(ui): implement SWR silent polling and incremental stream prepend to prevent flickering"
```

---

### Task 5: Frontend UI: IP Visitors Aggregated List & AI Stream Filters

**Files:**
- Modify: `web/static/js/modules/state.js`
- Modify: `web/static/js/translations.js`
- Modify: `web/static/js/modules/analytics.js`
- Modify: `web/components/project/TabVisitors.html`

**Interfaces:**
- Consumes: `GET /api/v1/projects/:id/visitors`, `GET /api/v1/projects/:id/visit-stream`
- Produces: Dual-view Visitors tab with IP List & Realtime Stream + AI badge filters

- [ ] **Step 1: Update `state.js` and `translations.js`**
In `state.js`:
- Add `visitorViewMode: 'stream'`, // 'stream' | 'ip'
- Add `streamFilter: 'all'`, // 'all' | 'human' | 'ai'
- Add `loadingVisitorsPage: false`.

In `translations.js`:
- Add translations: `viewRealtimeStream`, `viewIPVisitors`, `ipAddress`, `pvHits`, `firstSeen`, `lastSeen`, `recentPage`, `filterAll`, `filterHuman`, `filterAI`, `aiAgentBadge`, `aiVisitsCount`.

- [ ] **Step 2: Implement view mode switcher & IP table in `TabVisitors.html`**
- Segmented control at top:
  - `[ ⚡ 实时访问流水 (Realtime Stream) ]` (active when `visitorViewMode === 'stream'`)
  - `[ 👥 今日 IP 访问列表 (IP Visitors) ]` (active when `visitorViewMode === 'ip'`)
- In Stream view:
  - Filter pills: `[ 全部 ]` `[ 👤 人类访客 ]` `[ 🤖 AI 智能体 ]`
  - Tag AI records with violet badge `🤖 AI: OpenAI (GPTBot)`
- In IP Visitors view:
  - Table columns: IP (Masked), Location (Country/Region/City), Total PV (`item.requests`), First Seen, Last Seen, Recent Page, Device/Browser.
  - Pagination controls (Page X of Y, Previous, Next).

- [ ] **Step 3: Connect tab switching and data fetching in `analytics.js`**
When `visitorViewMode === 'ip'`, call `loadVisitors(currentProject.id)`.

- [ ] **Step 4: Verify JS syntax**
Run: `node -c web/static/js/modules/state.js && node -c web/static/js/translations.js && node -c web/static/js/modules/analytics.js`
Expected: Clean exit (code 0)

- [ ] **Step 5: Commit**
```bash
git add web/static/js/modules/state.js web/static/js/translations.js web/static/js/modules/analytics.js web/components/project/TabVisitors.html
git commit -m "feat(ui): add dual-view IP visitors table and AI agent stream filtering"
```

---

### Task 6: Frontend UI: Visited Pages Breakdown Table in TabOverview

**Files:**
- Modify: `web/components/project/TabOverview.html`
- Modify: `web/static/js/modules/formatters.js`
- Modify: `web/static/js/translations.js`

**Interfaces:**
- Consumes: `projectAnalysis.paths` or `projectStats.paths`
- Produces: Searchable, sortable visited pages breakdown table

- [ ] **Step 1: Add formatter helper and translations**
In `formatters.js`:
- Add `filterPages(pagesMap, searchQuery)` returning sorted array of `{ path, count, percentage }`.
In `translations.js`:
- Add `pagesBreakdown`, `pageRank`, `pagePath`, `pageViews`, `pageShare`, `searchPagesPlaceholder`.

- [ ] **Step 2: Render full Pages Breakdown Table in `TabOverview.html`**
- Replace the small top pages list with a full-width card:
  - Header: Title `受访页面明细 (Pages Breakdown)`, Total count badge, Path search input box (`x-model="pageSearchQuery"`).
  - Table:
    - 排名 (#1, #2, ...)
    - 受访页面路径 (Path, monospace)
    - 浏览量 (PV count, bold)
    - 占比 (Progress bar + percentage %)
    - 操作 (External link button with hover tooltip)

- [ ] **Step 3: Verify JS syntax & Run full test suite**
Run: `node -c web/static/js/**/*.js && go test ./...`
Expected: ALL PASS

- [ ] **Step 4: Commit**
```bash
git add web/components/project/TabOverview.html web/static/js/modules/formatters.js web/static/js/translations.js
git commit -m "feat(ui): add searchable visited pages breakdown table in TabOverview"
```

---

### Task 7: Full System Verification & Regression Testing

- [ ] **Step 1: Run complete Go test suite**
Run: `go test -v -race ./...`
Expected: 100% PASS

- [ ] **Step 2: Verify zero frontend console/syntax errors**
Run: `node -c web/static/js/**/*.js`
Expected: All files valid

- [ ] **Step 3: Manual sanity check with running dev server**
Test SVG counter request:
`curl -H "Referer: https://example.com/blog/test" http://localhost:8080/counter/{slug}/pv.svg`
Confirm PV, UV, IP increment and stream record created.
Test AI crawler request:
`curl -A "Mozilla/5.0 (compatible; GPTBot/1.2; +https://openai.com/gptbot)" http://localhost:8080/counter/{slug}/pv.svg`
Confirm AI visit recorded in stream with `isAiAgent: true`.
