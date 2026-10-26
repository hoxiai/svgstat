# Webmaster Analytics V2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give website owners source-separated, consistently ranged analytics with complete page reporting and clear coverage/freshness states.

**Architecture:** Continue collecting into Redis and flushing bounded daily aggregates into PostgreSQL. Add an explicit source-separated metric shape and a version/time-zone boundary for daily rows. Extend authorized report queries and use one shared period selection across the analytics workspace.

**Tech Stack:** Go 1.25.1, PostgreSQL, Redis, Gorilla Mux, Alpine.js templates, existing zero-build frontend.

**Spec:** `docs/superpowers/specs/2026-10-26-webmaster-analytics-v2-design.md`

## Global Constraints

- Redis remains the first write target; SVG hot paths do not access PostgreSQL.
- Do not persist raw IPs or raw browser event streams.
- Existing SVG URLs, public counter values, and API callers remain compatible.
- Keep the frontend build-free and use the existing Go/Redis/PostgreSQL stack.
- Every project report remains scoped to the authenticated owner.
- Legacy mixed statistics remain labeled as legacy; never invent source attribution for old rows.
- Custom date ranges are limited to 90 days; page size is limited to 100.

---

### Task 1: Persist explicit metric coverage, project time zone, and daily aggregation version

**Files:**
- Create `migrations/027_webmaster_analytics_v2.sql`.
- Modify `internal/project/project.go` and all project select/insert/update queries in `internal/project/postgres.go`.
- Modify `internal/api/handlers.go` project creation/settings payloads.
- Modify `internal/analytics/analytics.go` daily-stat fields and date-key selection.
- Modify `internal/worker/worker.go` flush dates and daily upsert.
- Modify `internal/metrics/metrics.go` report metadata.
- Modify corresponding project, analytics, worker, and metrics tests.

**Interfaces:** Project reads provide a validated IANA time zone. `DailyStats` distinguishes website pageviews from legacy aggregate PV and carries aggregation version/time-zone coverage. Reports return `startDate`, `endDate`, `timeZone`, `coverage`, and `lastUpdatedAt`.

- [ ] Add a migration that defaults existing projects to UTC, gives new projects `Asia/Shanghai`, preserves existing daily rows as version 1, and allows distinct version/time-zone rows for the same project/date.
- [ ] Add project-local date helpers that reject unknown zones and cover DST transitions.
- [ ] Write failing tests for UTC legacy rows, Asia/Shanghai day boundaries, invalid zones, and versioned same-date upserts.
- [ ] Run the focused tests and verify the expected failures.
- [ ] Implement the migration, project persistence, local date calculation, and worker upsert.
- [ ] Run focused tests and verify the new behavior.
- [ ] Update `DATABASE.md` and `ANALYTICS.md` with legacy coverage and daily bucket rules.

### Task 2: Separate SVG request collection from website pageview collection

**Files:**
- Modify `internal/analytics/analytics.go` and `internal/analytics/stream.go`.
- Modify SVG collection call sites in `internal/api/handlers.go`.
- Modify daily-stat fields/SQL arguments in `internal/worker/worker.go`.
- Modify `internal/api/svg_tracking_test.go`, `internal/analytics/ip_hourly_test.go`, and `internal/analytics/visit_attribution_test.go`.

**Interfaces:** SDK collection writes the website metric family. SVG collection writes request totals, `svgRequests`, and the named public counter only. Existing aggregate `pv` and `requests` fields remain available for legacy API clients.

- [ ] Add failing Redis-backed tests proving an SVG request increments neither website PV, UV, IP, paths, source totals, nor visit stream, while its named counter still increases.
- [ ] Add a failing test proving one SDK pageview still updates website PV/UV/IP and attribution, and a recognized bot does not.
- [ ] Run focused tests to verify the expected failures.
- [ ] Route SVG requests to the request-only collection path and isolate website aggregates by source/version.
- [ ] Extend worker persistence without replacing legacy columns or changing counter URLs.
- [ ] Run focused tests and verify new and compatibility behavior.
- [ ] Update `README.zh-CN.md`, `README.md`, `API.md`, and `ANALYTICS.md` to state the separate definitions.

### Task 3: Add consistent bounded date-range queries and full page breakdowns

**Files:**
- Modify `internal/api/routes.go`, `internal/api/handlers.go`, and `internal/api/conversion_handlers.go`.
- Modify `internal/metrics/metrics.go`, `internal/metrics/session_quality.go`, and `internal/metrics/issues.go`.
- Modify `internal/analytics/analytics.go` page aggregate readers.
- Modify `internal/metrics/metrics_test.go`, `internal/metrics/session_quality_test.go`, `internal/metrics/issues_test.go`, and API validation tests.

**Interfaces:** Existing `days=7|30|90` calls stay valid. New reports accept `start_date` and `end_date` in project local dates, reject inverted or greater-than-90-day ranges, return the preceding equal-length comparison period, and report coverage/freshness. A new authorized page endpoint accepts `start_date`, `end_date`, `page`, `page_size`, and `search`.

- [ ] Add failing range tests for each report family, including leap day, year boundary, invalid dates, DST boundary, and 90-day limit.
- [ ] Add failing page-query tests proving a 200-path data set can search and paginate beyond page 100 while shares use full-period website PV and expose unlisted volume.
- [ ] Run focused tests to verify the expected failures.
- [ ] Add one shared range parser and one project-time-zone range calculation; adapt historical, quality, issue, event, and funnel queries.
- [ ] Implement bounded server-side page search/pagination over retained daily dimensions with deterministic tie ordering and an explicit remainder.
- [ ] Run focused tests and verify all report families return identical selected/comparison dates.
- [ ] Document endpoint parameters and coverage/freshness response fields in `API.md`.

### Task 4: Build the shared period control and webmaster analytics navigation

**Files:**
- Modify `web/static/js/modules/state.js`, `web/static/js/modules/analytics.js`, and `web/static/js/modules/router.js`.
- Modify `web/components/project/ProjectHeader.html`, `web/components/project/TabOverview.html`, `web/components/project/TabGrowth.html`, `web/components/project/TabQuality.html`, and `web/components/project/TabVisitors.html`.
- Add source/page destination templates under `web/components/project/` and include them from `web/components/pages/DashboardProject.html`.
- Modify `web/static/js/translations.js` and `web/static/js/modules/formatters.js`.

**Interfaces:** One `selectedDateRange` state is read by every historical report loader. Realtime and hourly endpoints retain explicit fixed windows. Each page has independent loading/error/empty/freshness state and never renders failed requests as zero.

- [ ] Add failing frontend method checks for date-range request parameters, unchanged real-time windows, zero-denominator comparison labels, and stale-data preservation after a failed refresh.
- [ ] Run the focused frontend checks to verify expected failures.
- [ ] Add shared date-range controls and request version guards.
- [ ] Render source-separated overview metrics, sources, and a server-paginated pages table with complete shares and coverage text.
- [ ] Split sources and pages into dedicated destinations; expose existing growth and quality views as advanced analysis with the shared date range.
- [ ] Add translated states for collection pending, rejected, insufficient sample, stale, empty, and failed refresh.
- [ ] Run focused checks and inspect the local dashboard at desktop and mobile widths.

### Task 5: Complete compatibility and migration review

**Files:**
- Update `CONTRACTS.md`, `DATABASE.md`, `ANALYTICS.md`, `API.md`, and `README.zh-CN.md`/`README.md`.
- Review migration rollback/replay behavior in `internal/migrate/`.

- [ ] Verify every acceptance criterion in the spec against the API and UI.
- [ ] Verify existing `days` requests, project ownership, SVG URLs, and named counter values remain compatible.
- [ ] Verify the legacy banner starts and ends at the stored source-separation boundary and no query merges incompatible time zones.
- [ ] Run the complete Go test suite and SDK contract checks.
- [ ] Record skipped environment-dependent checks and any remaining cardinality limits in the handoff.
