# Baidu-Style Analytics Overhaul Design

## 1. Background & Motivation

Current analytics in SVGStat provides basic counter tracking and attribution, but webmasters find it underwhelming ("鸡肋") compared to standard analytics platforms like 百度统计 (Baidu Tongji):
1. **Missing Core IP Metric**: Webmasters benchmark traffic using the "Golden Trio": **PV (Page Views)**, **UV (Unique Visitors)**, and **IP (Unique Client IPs)**. The dashboard currently displays technical metrics (Requests, Bots) instead of IP.
2. **Lack of Comparative Context**: Metrics are isolated single numbers without comparison to yesterday's same-period or full-day performance, preventing quick judgment of traffic momentum.
3. **No 24-Hour Realtime Hourly Curve**: No hourly 0:00–24:00 comparison between today's live trend and yesterday's baseline.
4. **Visitor Tab is Not an Access Stream**: `TabVisitors` only stores aggregated visitor state (last seen, total hits) rather than a chronological, live-updating visit log stream showing who visited what page from which source.
5. **Traffic Sources Need Baidu-Style Stratification**: Webmasters expect clean three-layer categorization: **Search Engines (with keywords)**, **External Links (Referring URLs)**, and **Direct Traffic**.

This design overhauls SVGStat to provide a first-class, Baidu-style analytics experience while respecting our architectural invariants: Redis-first hot writes, zero slop, low-latency, and privacy compliance.

---

## 2. Architecture & Data Model

### 2.1 Core Metrics & IP Tracking

#### Redis Keys (Day-scoped, TTL: `72h`)
- `project:{id}:ipset:{date}` (Set): stores raw/hashed client IPs to compute daily unique IP count via `SCard`.
- `project:{id}:hourly:{date}:{metric}` (Hash): field `00`..`23`, value incremented on each hit for `pv`, `uv`, and `ip`.
  - For `uv` and `ip`, hourly increments occur only when the visitor/IP is first seen in that hour bucket (guarded by hourly sets `project:{id}:hourly_uvset:{date}:{hour}` and `project:{id}:hourly_ipset:{date}:{hour}` with 2h TTL).

#### PostgreSQL Schema Migration (`025_add_daily_statistics_ip.sql`)
```sql
ALTER TABLE daily_statistics
ADD COLUMN IF NOT EXISTS ip BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS hourly JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN daily_statistics.ip IS 'Unique IP count for the day';
COMMENT ON COLUMN daily_statistics.hourly IS 'Hourly distribution: {pv: {"00": 10, ...}, uv: {...}, ip: {...}}';
```

#### Worker Persistence
The background worker extracts `ip` (`SCard` on `ipset`) and `hourly` breakdown from Redis and upserts them into `daily_statistics`.

---

### 2.2 Comparative Benchmark (Today vs Yesterday)

When serving `GetTodayOverview`:
1. **Today Live Data**:
   - `pv`, `uv`, `ip` from Redis.
   - Hourly distribution `00` up to current hour.
2. **Yesterday Full-Day Data**:
   - `pv`, `uv`, `ip` from PostgreSQL / Redis.
3. **Yesterday Same-Period Baseline**:
   - Sum of yesterday's hourly buckets from `00` to current hour (plus prorated fraction of the current hour).
   - Compute percentage difference: `(today - yesterday_same_period) / yesterday_same_period * 100%`.

---

### 2.3 Realtime Visit Stream (访问明细流水)

#### Redis Structure: Bounded Stream List
- Key: `project:{id}:visit_stream` (List)
- Operations on every valid pageview:
  - `LPUSH project:{id}:visit_stream <JSON>`
  - `LTRIM project:{id}:visit_stream 0 499` (capped at 500 recent items)
  - `EXPIRE project:{id}:visit_stream 172800` (48 hours)

#### Stream Item Payload
```json
{
  "id": "1727531234000-a1b2",
  "timestamp": "2026-09-28T21:52:10Z",
  "time_str": "21:52:10",
  "masked_ip": "114.240.*.*",
  "country": "中国",
  "region": "北京",
  "city": "北京",
  "source_category": "search",
  "source_name": "百度搜索",
  "source_url": "",
  "search_keyword": "svgstat 统计组件",
  "path": "/pricing",
  "page_title": "价格方案",
  "device_type": "desktop",
  "browser": "Chrome 128",
  "os": "Windows 11"
}
```

#### API Endpoint
- `GET /api/projects/{id}/visit-stream?limit=50&since=<optional_id>`
  - Returns recent stream items for live table rendering and auto-polling (every 5–10s).

---

### 2.4 Baidu-Style Traffic Sources Stratification

Refactor sources presentation into 3 canonical groups:
1. **Search Engines (搜索引擎)**:
   - Engines: `Baidu (百度)`, `Google`, `Bing (必应)`, `360 (奇虎360)`, `Sogou (搜狗)`, `Shenma (神马)`, `Toutiao (头条搜索)`, `Other Search`.
   - Metrics: Visits, Share %, Search Keywords extracted from referrers/UTM.
2. **External Referrals (外部链接)**:
   - Grouped by Referring Domain (e.g., `v2ex.com`, `github.com`, `zhihu.com`).
   - Detailed Referring URLs: full URL preserved when safe, rendered with external click-through icon so webmasters can verify incoming links.
3. **Direct Traffic (直接访问)**:
   - Bookmarks, direct URL entries, untracked mobile apps.
4. *(Auxiliary: AI Assistants like ChatGPT/DeepSeek/Kimi, Social Media, Paid Ads)*.

---

### 2.5 Dashboard UI Upgrades (`TabOverview.html` & `TabVisitors.html`)

1. **Today Overview Card**:
   - Three hero metrics: **Today PV**, **Today UV**, **Today IP**.
   - Sub-badges under each metric:
     - `vs Yesterday Same Period: ↑ 15.2%` (green for up, red for down).
     - `Yesterday Full Day: 1,520`.
     - Secondary metrics: Average PV/UV, Average Session Duration.
2. **24-Hour Comparison Chart**:
   - Dual-line graph from 0:00 to 24:00:
     - **Solid Line**: Today's hourly trajectory (live, stops at current hour).
     - **Dashed Line**: Yesterday's 24h baseline.
     - Toggle selector for: `[ PV ] [ UV ] [ IP ]`.
3. **Traffic Sources & Search Engines Section**:
   - Donut / Stacked bar for Source Categories (Search Engine %, External Links %, Direct %, Other %).
   - Search Engine leaderboard table with search terms.
   - Top Referring Pages table with clickable external URLs.
4. **Tab "访客明细" (Visitors) -> "实时访问明细" (Realtime Stream)**:
   - Chronological table with live streaming feeling.
   - Columns: Time, IP & Region, Source / Keyword, Visited Page, Device & OS.
   - "Auto Refresh" switch (every 5 seconds) with subtle indicator.

---

## 3. Privacy & Performance Safeguards

1. **Privacy**: Raw IPs are masked (`114.240.*.*`) before entering the visit stream list. Only aggregate counts are persisted to PostgreSQL `daily_statistics`.
2. **Bounded Memory**: `visit_stream` is hard-capped at 500 elements via `LTRIM`.
3. **Zero Hot-Path Blocking**: Stream push and hourly increments use Redis pipeline; no synchronous database writes in the request ingestion pipeline.

---

## 4. Verification & Testing Strategy

1. **Unit & Pipeline Tests**:
   - `internal/analytics/analytics_test.go`: verify `ipset` increments, hourly increments, and `visit_stream` LPUSH/LTRIM bounds.
   - `internal/metrics/metrics_test.go`: verify same-period calculation, 24h hourly points, and IP aggregation.
   - `internal/worker/worker_test.go`: verify persistence of `ip` and `hourly` fields to database.
2. **API & End-to-End Tests**:
   - `GET /api/projects/{id}/stats/today`: validates response contains `ip`, `yesterday_same_period`, `yesterday_full`, and `hourly`.
   - `GET /api/projects/{id}/visit-stream`: validates formatted JSON stream items.
3. **Frontend Verification**:
   - Run local dev server, ingest test requests with various referrers (Baidu, Bing, external URLs), verify 24h hourly chart and realtime stream table updates.
