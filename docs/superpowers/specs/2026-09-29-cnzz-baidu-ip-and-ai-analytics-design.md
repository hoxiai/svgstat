# CNZZ & Baidu-Style IP, PV, UV, Page Breakdown, AI Detection & Anti-Flicker Design

## 1. Background & Goals

Webmasters accustomed to platforms like **CNZZ (友盟)** and **百度统计 (Baidu Tongji)** expect:
1. **Core Trio (Golden Trio)**: Immediate visibility into **Today's PV, UV, and IP**, complete with yesterday same-period and yesterday full-day baselines, and a 24-hour dual-line curve for PV, UV, and IP.
2. **SVG Counter & Badge Tracking**: When badges and counters are embedded in blogs, websites, or GitHub READMEs, every hit must contribute to PV, UV, IP, hourly distributions, and live access logs.
3. **IP Access Record List (今日 IP 访问明细)**: An aggregated view of today's visitors grouped by IP, showing IP location, total page views (hits), first and last seen timestamps, last visited page, and client device.
4. **Visited Pages Breakdown (受访页面明细)**: A clear, sortable, searchable table showing top visited URLs/paths, page views (PV), and traffic share %.
5. **Clear Traffic Sources Stratification**: Direct traffic, external referral links (with full URL link-out), and search engines (with keywords).
6. **AI Visit & Agent Detection (AI 访问探测)**: Modern webmasters want visibility into AI traffic:
   - **AI Referrals**: Human visits referred by AI platforms (ChatGPT, Claude, Perplexity, DeepSeek, Kimi, Doubao, etc.).
   - **AI Agents / Crawlers**: Autonomous AI bots and search agents (GPTBot, ChatGPT-User, ClaudeBot, PerplexityBot, Bytespider, DeepSeekBot, Applebot-Extended, etc.) crawling pages or fetching counters. Display clear `[AI 智能体]` badges and provide a stream filter `[全部 / 人类访客 / 🤖 AI 智能体]`.
7. **Zero-Flicker UI (流畅无闪烁刷新)**: Background polling (5s stream, 30s stats) must use SWR (Stale-While-Revalidate) in-place updates. No data zeroing, no full-card spinners, and incremental prepending of stream records without tearing down table DOM nodes.

---

## 2. Architecture & Data Flow

```
                      Browser / Crawler Request
                                 │
                 ┌───────────────┴───────────────┐
                 │                               │
        JS Tracker / Collect             SVG Badge / Counter
        (/api/v1/collect)              (/counter/... or /badge/...)
                 │                               │
                 │                               ▼
                 │                      TrackBadgeOrCounter
                 │               (countRequest: true, countPageview: true)
                 │               (Path extracted from Referer / URL)
                 │                               │
                 └───────────────┬───────────────┘
                                 ▼
                     extractRequestData & AI Detection
                     - detectAICrawler(userAgent)
                     - applyAttribution(websiteAttribution)
                                 │
                                 ▼
                         Redis Hot Pipeline
       ┌─────────────────────────┼─────────────────────────┐
       ▼                         ▼                         ▼
  Metric Counters           24h Hourly Buckets        Visit Stream & IP
  - pvKey, requestsKey      - hourly:pv/uv/ip         - visit_stream (capped 500)
  - ipset (unique IPs)      - hourly_uvset/ipset      - visitor_v2 (IP records)
  - ai_visits (today AI)                               - visitors_v2 (index)
```

---

## 3. Detailed Specifications

### 3.1 SVG Request Tracking & Attribution (`internal/analytics/analytics.go`, `internal/api/handlers.go`)

In `handleCounterSVG` and `handleBadgeSVG`:
- Currently, they call `a.analytics.TrackRequest(r.Context(), r, proj.ID)`.
- `TrackRequest` sets `countPageview: false`, which skips PV, UV, IP, hourly buckets, visitor records, and `pushVisitStream`.
- **Change**: Introduce `TrackBadgeOrCounter(ctx context.Context, req *http.Request, projectID string) error`:
  - Extracts referrer via `req.Referer()`. If referrer has a valid path (e.g. `https://userblog.com/posts/hello`), uses that path as `Path`. If referrer is empty or has no path, falls back to `req.URL.Path`.
  - Applies attribution via `websiteAttribution(path, referrer, originHost)`.
  - Calls `trackRequestData(ctx, data, true, true)` (`countRequest: true, countPageview: true`).
  - Thus, SVG badge/counter loads are fully counted as real pageviews with IP tracking, visitor logging, and stream insertion.

### 3.2 AI Visit & Agent Detection (`internal/analytics/ai_detector.go`, `internal/analytics/stream.go`)

#### 3.2.1 AI Crawler & Agent Detection
Define `detectAICrawler(userAgent string) (isAI bool, aiName string)`:
- `GPTBot`: `OpenAI (GPTBot)`
- `ChatGPT-User`: `OpenAI (ChatGPT-User)`
- `OAI-SearchBot`: `OpenAI (SearchBot)`
- `ClaudeBot`, `Claude-Web`, `anthropic-ai`: `Anthropic (ClaudeBot)`
- `PerplexityBot`: `Perplexity`
- `Bytespider`: `字节跳动 (豆包/Bytespider)`
- `DeepSeekBot`: `DeepSeek (深度求索)`
- `Google-Extended`: `Google (Gemini)`
- `Applebot-Extended`: `Apple (Applebot)`
- `Meta-ExternalAgent`: `Meta (Llama)`
- `cohere-ai`: `Cohere`
- `Moonshot`, `KimiBot`: `Moonshot (Kimi)`
- `YouBot`: `You.com`

#### 3.2.2 AI Stream Payload & Tracking
In `VisitStreamItem`:
- `IsAIAgent bool `json:"isAiAgent"``
- `AIName string `json:"aiName,omitempty"``
- When `detectAICrawler` matches:
  - `IsAIAgent = true`
  - `AIName = aiName`
  - `SourceCategory = "ai"`
  - `SourceName = aiName`
  - `DeviceType = "bot"`
  - `Browser = aiName`
  - Redis pipeline increments `project:{id}:ai_visits:{date}` with key TTL.
- When an AI crawler visits, it is allowed into `pushVisitStream` with `isAiAgent: true` so webmasters can see AI visits in realtime!
- Non-AI generic bots (e.g. `curl`, `python-requests`, `wget`, `crawler`) remain excluded from stream and PV/UV to prevent junk pollution.

### 3.3 Today Overview AI Metrics (`internal/metrics/today_overview.go`)
In `TodayOverview`:
- Add `TodayAI int64 `json:"todayAi"``
- Reads `project:{id}:ai_visits:{date}` from Redis.
- Returns `todayAi` so the frontend can display AI visits and badges.

### 3.4 IP Access Record List (`GET /api/v1/projects/:id/visitors`) & UI Toggle
The backend already provides `GetTodayVisitors` returning paginated `VisitorDetail` items (`ip`, `country`, `region`, `city`, `requests`, `firstSeenAt`, `lastSeenAt`, `path`, `deviceType`, `browser`).
- In `TabVisitors.html`:
  - Add segmented control:
    `[ ⚡ 实时访问流水 (Realtime Stream) ]` | `[ 👥 今日 IP 访问列表 (IP Visitors) ]`
  - When in IP Visitors mode:
    - Renders the aggregated IP table.
    - Columns:
      - 访客 IP (Masked IP, e.g. `114.240.23.*`)
      - 地理位置 (国家 / 省 / 市)
      - 浏览量 (PV 贡献次数, `item.requests`)
      - 首次访问时间 (`firstSeenAt`)
      - 最近访问时间 (`lastSeenAt`)
      - 最近受访页面 (`path`)
      - 终端与浏览器 (`deviceType`, `browser`)
    - Pagination controls (Previous / Next page).
  - When in Realtime Stream mode:
    - Filter pills: `[ 全部 ]` `[ 👤 人类访客 ]` `[ 🤖 AI 智能体 ]`
    - Displays AI Agent visits with a distinctive violet badge `[🤖 AI 智能体: OpenAI (GPTBot)]`.

### 3.5 Visited Pages Breakdown Table (`TabOverview.html`)
In `TabOverview.html`:
- Transform the "受访页面" card into a dedicated, full-width table:
  - Search input box to quickly filter paths.
  - Columns:
    - 排名 (#1, #2, ...)
    - 受访页面路径 (Path)
    - 浏览量 (PV)
    - 占比 (Progress bar + percentage %)
    - 操作 (外部链接新窗口跳转查看)
  - Supports sorting by PV descending.

### 3.6 Frontend Anti-Flicker SWR Architecture (`analytics.js`, `formatters.js`)
1. **Silent Refresh Flag (`isSilent = true`)**:
   - `loadProjectOverview(id, isSilent)`
   - `loadStats(id, isSilent)`
   - `loadTrend(id, isSilent)`
   - `loadAnalysis(id, isSilent)`
   - `loadVisitStream(id, isSilent)`
   - During background polling (5s for stream, 30s for overview), pass `isSilent: true`:
     - Do NOT set `loadingStats = true` or `loadingVisitStream = true`.
     - Do NOT reset data models to empty (`createEmptyProjectStats()`, etc.).
     - Replace data in place once the response arrives.
2. **Incremental Stream Prepending**:
   - In `loadVisitStream`:
     - Given new items array `incoming`:
     - Compare against existing IDs: `const existingIds = new Set(this.visitStream.map(i => i.id));`
     - Find new records: `const newItems = incoming.filter(i => !existingIds.has(i.id));`
     - If `newItems.length === 0`, do nothing (no Alpine DOM churn!).
     - If `newItems.length > 0`, prepend: `this.visitStream = [...newItems, ...this.visitStream].slice(0, 50);`.

---

## 4. Invariants & Security
1. **Privacy**: Full client IPs are never stored in PostgreSQL. Ephemeral Redis sets use raw IPs solely for atomic `SCard` deduplication (48-72h TTL). All stream logs and visitor records store masked IPs (`114.240.*.*`).
2. **Performance**: Zero DB queries on the hot path. AI crawler detection is compiled via simple substring/keyword checks (`O(1)` or fast small slice scan).
3. **Zero Build**: Frontend strictly adheres to vanilla ES modules + Alpine.js + UnoCSS without npm build steps.
