# Webmaster Analytics V2 Design

## Goal

Make SVGStat a clear, trustworthy analytics workspace for website owners. A site owner should be able to tell whether traffic changed, where it came from, which pages people viewed, and whether collection has problems.

## Product boundary

Website SDK events are the source of website pageviews, visitors, attribution, and page-level reporting. SVG badge and counter requests remain a separate request metric and continue updating their own public counters. They must not be presented as website pageviews or visitors. AI referrers describe a human traffic source; identified AI crawlers are machine traffic and remain separate from human visitor metrics.

Existing daily rows contain mixed SDK and SVG counts and cannot be reliably split. Keep them readable as legacy data, identify their mixed coverage in API responses, and begin source-separated coverage on the first day the new pipeline records. Do not relabel or redistribute historical counts.

## Information architecture

The project workspace uses five destinations:

1. **Overview**: selected-period pageviews, anonymous visitors, unique IPs, comparison with the previous equal period, a main trend, leading sources and pages, and qualified issues.
2. **Sources**: search, referrals, direct, social, campaign, and AI referrals, with landing-page detail.
3. **Pages**: paginated and searchable page list with views, share of all tracked views, period change, and entrance count where the existing aggregates support it.
4. **Live**: a fixed recent window and the latest access stream. Identify known crawlers as machines; label unclassified requests as unknown rather than human.
5. **Settings and diagnostics**: SDK instructions, allowed domains, collection status, and safe test traffic.

Conversion and session-quality tools remain available as advanced analysis and use the same selected period. SVG embed code remains accessible independently of website tracking.

## Metric definitions

- `websitePageviews`: non-bot SDK pageview events only.
- `anonymousVisitors`: daily unique browser/session identifiers, not verified people. In a multi-day range, label the total as visitor-days because daily uniques are summed; do not call this a globally deduplicated visitor count.
- `uniqueIPs`: daily unique IPs counted from eligible SDK pageviews; range totals are IP-days for the same reason.
- `svgRequests`: successful badge and counter image requests. These do not increment website pageviews, visitors, IPs, or website attribution.
- `periodComparison`: current selected period against the immediately preceding period of equal length. A zero or unavailable denominator returns an unavailable comparison, not 0%.
- `pageShare`: page views divided by all website page views in the same selected period. Return the full denominator and an explicit unattributed/unlisted remainder when dimension caps prevent a complete page list.
- `todayHourly`: today's project-local hour buckets compared only with yesterday's matching completed hours. Do not estimate UV/IP from ratios; label incomplete current-hour data and unavailable historical buckets.
- Every report response includes its start/end dates, project time zone, data freshness, metric coverage (`legacy_mixed` or `source_separated`), and any sampling or cardinality limitations.

## Date and time

Add an IANA time-zone setting to each project. Newly created projects default to `Asia/Shanghai`; existing projects retain `UTC` until an owner changes it. Validate time-zone identifiers. Apply the selected project time zone consistently to event-day buckets, worker flush dates, history queries, stream display, and comparisons. An owner changing the time zone starts a new aggregation coverage boundary; keep earlier rows labeled with their original zone and do not merge the two series silently. The workspace offers today, yesterday, 7 days, 30 days, 90 days, and a bounded custom range. The selected period is shared by historical reports; real-time and hourly views name their fixed windows.

## Data and API behavior

- Keep Redis as the hot write target and PostgreSQL as the daily historical store. Do not add raw-event retention or a new analytics datastore.
- Preserve existing endpoints and their existing `pv`/`requests` meanings for compatibility. Add explicit source-separated fields and metadata; the new workspace uses the explicit website fields.
- Add a project-scoped pages query with bounded page size, server-side search, deterministic ordering, full-period denominator, and pagination metadata. Do not calculate shares from a truncated top-N list.
- Add date-range query support to analytics, session-quality, issue, event, and funnel reports. Keep existing `days=7|30|90` callers working.
- Store a version and time-zone boundary for daily rows so the worker can persist new project-local aggregates without rewriting mixed legacy rows. Project-scoped authorization remains enforced by existing handlers.
- SVG request tracking increments its request counters only. SDK pageviews continue writing website metrics and attribution. Public SVG URLs and counter values remain compatible.

## Empty, stale, and error states

The UI distinguishes: no events collected, tracking not installed, collection rejected by configuration, insufficient sample size, stale data, and a failed API request. Preserve prior data during background refresh and expose the last successful update time. Never show a request failure as a successful zero.

## Privacy and performance

Keep the current no-raw-IP PostgreSQL policy, bounded Redis cardinality, project ownership checks, and Redis-first collection path. Do not add full visitor identity, session replay, or raw event storage. Bound custom date ranges to 90 days and page sizes to 100.

## Scope and migration

This version updates source-separated collection, daily persistence, report queries, and the project analytics UI. Existing user APIs remain backward compatible. Existing mixed rows are marked as legacy coverage; the new UI avoids comparing legacy mixed counts to source-separated counts without an explicit coverage boundary. Documentation and English/Chinese interface text must use the definitions above.

## Acceptance criteria

1. Loading an SVG counter or badge increases `svgRequests` and its named counter without increasing website pageviews, anonymous visitors, unique IPs, or website source/page totals.
2. SDK pageviews update website pageviews and website attribution once per event, excluding identified bots.
3. Page shares use the complete selected-period website pageview total; search and pagination reach all retained page dimensions, and the response reports any unlisted remainder.
4. Range changes update all historical reports together. The response and UI show project-local dates, comparison dates, freshness, and coverage.
5. An unavailable baseline, a collection gap, an empty result, and a failed refresh have distinct UI states.
6. Existing SVG URLs, named counters, auth/project ownership, Redis-first writes, and clients using `days=7|30|90` remain compatible.
