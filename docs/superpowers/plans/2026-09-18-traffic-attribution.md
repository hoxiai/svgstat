# Traffic attribution overhaul

Goal: make traffic sources trustworthy (classification, paid clicks, self-referrals),
count them per visit, show realtime sources/pages, and capture the keywords that are
actually obtainable (utm_term, the rare search referrer that still carries a query,
and on-site search terms).

Out of scope: Google Search Console / Bing Webmaster integration (needs OAuth, token
storage and a sync job — separate project). Badge (SVG) requests keep counting only
`requests`; they carry no usable source.

## Task 1 — Attribution rules (`internal/analytics/attribution.go`)

- `websiteAttribution(rawPath, rawReferrer, siteHost string) Attribution` returning
  `{Path, Referrer, Source, Medium, Campaign, Term}`; exported wrapper keeps one entry
  point for the collect handler's diagnostics.
- Referrer host rules, matched by exact host / dot-suffix (never substring), in order:
  email → AI assistants → search engines (brand + any short TLD for google/yandex/yahoo) →
  social/community → known referral sites (github, stackoverflow, ...). Canonical source
  names (`google`, `baidu`, `bing`, `x`, `zhihu`, `chatgpt`, ...); unknown hosts keep the
  host without a leading `www.`.
- Mediums: `organic`, `cpc`, `social`, `ai`, `email`, `referral`, `none`.
- Paid click IDs on the landing URL (`gclid`, `gbraid`, `wbraid`, `msclkid`, `bd_vid`,
  `gdt_vid`, `qz_gdt`, `ttclid`, `twclid`, `li_fat_id`, `yclid`, `epik`) → vendor + `cpc`;
  `fbclid` → `facebook` + `social`. Explicit UTM always wins.
- UTM: normalize `utm_source` through the same aliases (`twitter`→`x`, host → canonical);
  normalize common `utm_medium` synonyms (`ppc`/`paid`/`sem` → `cpc`, `e-mail`/`newsletter`
  → `email`, `social-media`/`sns` → `social`). Capture `utm_term`.
- Term fallback: search-engine referrers that still carry `q`/`wd`/`word`/`query`/`p`/`text`.
- Self-referral: referrer host equal to the collecting site's host (from `Origin`,
  ignoring `www.`) is treated as no referrer.
- Tests: table test in `attribution_test.go` covering every row of the audit plus the
  existing `privacy_test.go` expectations (updated to canonical names).

## Task 2 — SDK

- Carry `utm_term`, `utm_content` and the click IDs from the landing URL to later
  pageviews (same mechanism as today's utm_source/medium/campaign).
- Send `search` = first present of `q, s, search, query, keyword, keywords, wd, kw,
  search_query` on the current URL; override with `data-search-params="a,b"`.

## Task 3 — Per-visit counting + new dimensions

- Move referrer/source/medium/campaign/term increments into `updateSessionScript`, run
  only when a new session starts (first pageview of a visit). Add a `channels` hash with
  `medium\x1fsource` fields for channel → source drill-down.
- `site_search` hash counted per pageview that carries a search term.
- Migration 024: `daily_statistics.terms`, `channels`, `site_searches` JSONB.
- Worker upsert, `GetStats`, metrics `GetAnalysis` breakdowns (`terms`, `channels`,
  `siteSearches`) and `maxDailyStats`.
- Tests: Redis-backed test that two pageviews in one visit count the source once and a
  new visit counts again; worker args include the new columns.

## Task 4 — Realtime sources and pages

- Per-minute hash `realtime:visits:{minute}` field=visitor, value=`source\x1fpath`.
- `GetRealtimeStats` merges the last 30 minutes (latest entry per visitor wins) into
  `sources` and `pages` visitor counts.
- Test: Redis-backed, two visitors / three pageviews → expected counts.

## Task 5 — Dashboard

- Overview: "Traffic sources" card with channel filter (All + mediums, translated) and
  sources for the selected channel; "Referring pages" card; "Search keywords" and
  "Site search" cards; realtime card gains top sources / active pages lists.
- Label note: source numbers are visits.
- i18n (zh/en) for all new strings.

## Verification

`go vet ./...`, `go test ./...` (Redis/PG available locally), migrate local DB, run the
app and send collect requests covering google/gclid/self-referral/site-search, then check
the dashboard in the browser.

Known behaviour change: rows written before this change count sources per pageview;
rows after count per visit. Periods spanning the switch mix both.
