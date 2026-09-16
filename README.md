# SVGStat

> Turn every visit into a visible signal.

[中文说明](./README.zh-CN.md)

SVGStat is a developer-first SVG analytics platform built with Go for the places where software gets judged in public: GitHub READMEs, docs, landing pages, changelogs, and internal dashboards.

Instead of asking you to wire up tracking scripts, screenshots, or custom widgets, SVGStat turns metrics into live SVG endpoints. You ship a URL, embed it like an image, and instantly make your project look active, trusted, and alive.

## Preview

![SVGStat Dashboard Preview](./preview.png)

## Why SVGStat

Most analytics products are built for websites you fully control.

SVGStat is built for high-visibility surfaces developers use to win trust:

- GitHub README files
- Markdown documents
- Documentation portals
- Static websites
- Open source project pages
- Internal engineering dashboards

It combines presentation and analytics in one workflow:

- Publish live counters as embeddable SVGs
- Generate polished badges that match your project style
- Attribute GitHub embeds with `page_id` when referrers are hidden
- Track PV, UV, referrers, countries, devices, browsers, and recent visitors
- Manage multiple projects and copy production-ready embed code with live preview

## What You Get

### Live SVG Counters

Publish SVG image request counts as lightweight counters that work anywhere Markdown or an `<img>` tag is supported. These counts represent image endpoint requests, not authoritative GitHub visitors, downloads, or stars.

### Brand-Ready SVG Badges

Generate production-ready badges with configurable labels, colors, styles, and clickable homepage links for README files, docs, product pages, and status surfaces.

### Actionable Analytics Workspace

See what is actually happening behind every badge and counter through a focused SPA dashboard covering:

- Page views
- Unique visitors
- Referrers and visited pages
- Countries
- Devices
- Browsers
- Anonymous visitor details (raw IP addresses are not stored)

### GitHub-Friendly Attribution

GitHub often hides the original referring page behind its image proxy. SVGStat supports `page_id` so you can still attribute README and Markdown embeds to the correct page and keep your dashboard useful.

### Shared Free Badge Node

SVGStat also includes a shared no-signup badge node for quick public usage. Each `page_id` gets an independent counter, which makes it ideal for lightweight GitHub visitor badges.

## Live Demo

- Product site: [https://svgstat.com](https://svgstat.com)
- Shared free badge: `https://svgstat.com/svg/free/badge/visitor.svg?label=visitors&page_id=github.com/hoxiai/demo`
- Demo project slug: `demo`
- Counter endpoint: `https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/hoxiai/demo`
- Badge endpoint: `https://svgstat.com/svg/demo/badge/requests.svg?label=Requests&color=0ea5e9&style=flat&page_id=github.com/hoxiai/demo`
- Markdown embed:

```markdown
![Visits](https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/hoxiai/demo)
```
![Visits](https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/hoxiai/demo)

## Quick Start

### Requirements

- Go `1.25.1`
- Podman or Docker
- Podman Compose or Docker Compose

### 1. Prepare environment

```bash
cp .env.example .env
```

### 2. Start PostgreSQL and Redis

```bash
make up
```

### 3. Apply database migrations

```bash
make migrate-up
```

### 4. Start the app

Use hot reload:

```bash
make watch
```

Or run the API directly:

```bash
go run cmd/api/main.go
```

### 5. Open the app

- SPA: [http://localhost:8080](http://localhost:8080)
- Health check: [http://localhost:8080/health](http://localhost:8080/health)
- Readiness check: [http://localhost:8080/ready](http://localhost:8080/ready)
- Prometheus metrics: [http://localhost:8080/metrics](http://localhost:8080/metrics)

### Optional: seed local test data

```bash
go run scripts/init_test_data.go
```

## API And Embed Examples

### Website Analytics

Add one script before `</head>` on a regular HTML site, WordPress theme, or SPA. Replace `my-project` with the project slug shown in the dashboard:

```html
<script defer src="https://svgstat.com/sdk.js" data-project="my-project"></script>
```

The SDK records the initial page view and automatically follows History API, back/forward, query-string, and hash route changes. It uses no cookies; an anonymous visitor identifier is kept in `sessionStorage`. Call `window.svgstatTrack('/virtual-page')` for custom virtual routes.

With no extra code, delegated listeners also capture four practical behaviors on current and dynamically inserted elements: `outbound_click`, `file_download`, `contact_click`, and `form_submit`. Automatic URL details contain only hostname and pathname—never query strings or hashes. Contact events contain only `email` or `phone`, and form events never read field values or button text.

The same automatic mode measures real-user LCP, INP, and CLS with the browser Performance Observer API, plus categorized JavaScript and resource-loading failures. The dashboard applies the standard Core Web Vitals thresholds and shows affected anonymous visitors. Error messages, promise values, stack traces, DOM selectors, and URL query/hash values are never collected. Error reporting is capped at 20 events per page load to contain failure storms; unsupported browsers are skipped safely.

Add a declarative custom event when a business action needs a name or explicit dimensions:

```html
<button data-svgstat-event="signup">Sign up</button>
<button data-svgstat-event="purchase" data-svgstat-value="99" data-svgstat-currency="CNY" data-svgstat-property-plan="pro">Buy</button>
```

Use `data-svgstat-ignore` on a container to exclude its behavior events. Add `data-auto-track="false"` to the SDK script to disable automatic behavior and quality monitoring while retaining page views and manual events. Values in `data-svgstat-property-*` are intentionally configured by your site and must not contain personal or sensitive data.

Product actions and optional conversion value can also be sent from JavaScript:

```js
window.svgstat('event', 'signup')
window.svgstat('event', 'purchase', { value: 99, currency: 'CNY' })
```

The dashboard can turn events into conversion goals and ordered funnels. It shows daily event trends and compares conversion or funnel completion by source, medium, campaign, page, device, and country. Conversion rate is daily anonymous converters divided by the matching daily anonymous audience, summed across the selected period. Source, medium, and campaign attribution use the session's first landing URL and referrer; raw event streams and long-lived identities are not stored.

Visit quality reports add 30-minute sessions, bounce rate, pages per session, estimated engaged time, entry and exit pages, aggregated adjacent page flows, and first-touch channel/device comparisons. A bounce is a one-page session. Engaged time only sums intervals between page views, so SVGStat does not invent dwell time for single-page visits. Page flows are stored as aggregate edges rather than replayable visitor journeys.

For installation checks, use test mode. Test traffic appears in the live debugger for 30 minutes but never changes production analytics:

```html
<script defer src="https://svgstat.com/sdk.js" data-project="my-project" data-mode="test"></script>
```

Website events are accepted by `POST /api/v1/collect`. Configure exact domains, wildcard subdomains such as `*.example.com`, or local development origins in the dashboard. An empty domain list allows any HTTP(S) origin for quick setup; restrict it before production.

### Counter SVG

```text
GET https://svgstat.com/svg/{projectSlug}/counter/{name}.svg
```

Example:

```text
https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=brightgreen
```

### Badge SVG

```text
GET /svg/{projectSlug}/badge/{name}.svg
```

Example:

```text
https://svgstat.com/svg/demo/badge/requests.svg?label=Requests&style=flat
```

### Project Statistics

```text
GET /api/v1/projects/{id}/stats
```

### Historical Trend

```text
GET /api/v1/projects/{id}/stats/trend?days=30
```

`days` accepts `7`, `30`, or `90`. The response includes continuous daily PV, UV, SVG request, and bot request series.

### Realtime And Period Analysis

```text
GET /api/v1/projects/{id}/stats/realtime
GET /api/v1/projects/{id}/analysis?days=30
GET /api/v1/projects/{id}/session-quality?days=30
GET /api/v1/projects/{id}/issues?days=30
```

Realtime statistics cover page views and unique visitors in the last 5 and 30 minutes. Period analysis compares the selected 7, 30, or 90 days with the immediately preceding period and returns page, referrer, country, device, browser, UTM source, medium, and campaign breakdowns. Session quality uses the same ranges and returns entry/exit pages, page flows, and channel quality. Period UV is the sum of daily unique visitors; page breakdowns omit query parameters to reduce sensitive-data exposure and high cardinality.

The issue report turns those aggregates into prioritized actions without storing raw events or visitor journeys. To limit false positives, traffic-drop alerts require at least 100 previous-period page views; conversion alerts only evaluate configured goals with at least 50 previous-period visitors and 5 converters; Core Web Vitals require 20 current samples. Collection-stop alerts require prior traffic and at least two inactive days. JavaScript and resource alerts use the percentage of anonymous visitors affected, not raw error volume alone.

### Installation Status

```text
GET /api/v1/projects/{id}/installation
```

Returns `pending` until SVGStat receives the project's first real website, counter, or badge request, then returns `installed` with the first and latest request timestamps.

Dashboard previews append `preview=1`. Preview requests render the SVG without incrementing counters, recording analytics, or marking the project as installed. Use the generated URL without `preview=1` in public embeds.

### Authentication

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

## Architecture

SVGStat keeps rendering, analytics, and project management in one focused Go service with a lightweight SPA frontend.

```text
README / Docs / Websites / Dashboards
                │
                ▼
          SVGStat HTTP Layer
        ┌───────┼────────┐
        ▼       ▼        ▼
      SPA     API    SVG Renderer
                │
                ▼
              Redis
                │
                ▼
           PostgreSQL
```

Design principles:

- Performance first
- Redis-first event handling
- Stateless SVG rendering
- API-first product design
- Clear separation between rendering and analytics

## Project Structure

```text
cmd/
  api/        # API server entrypoint
  migrate/    # migration CLI

internal/
  analytics/  # statistics aggregation
  api/        # routes and handlers
  auth/       # auth and session logic
  cache/      # cache layer
  config/     # configuration loading
  counter/    # counter SVG generation
  database/   # database bootstrap
  geoip/      # GeoIP lookup
  migrate/    # migration runner
  project/    # project data access
  renderer/   # shared SVG rendering
  worker/     # analytics flush worker

migrations/   # SQL migrations
scripts/      # helper scripts
web/          # Alpine.js SPA frontend
resource/     # static resources
```

## Tech Stack

- Go `1.25.1`
- PostgreSQL `16`
- Redis `7`
- Gorilla Mux
- pgx `v5`
- go-redis `v9`
- Alpine.js
- UnoCSS Runtime

## Admin Console

The admin console provides platform overview, user activation controls, and project status/capability management without entering the SVG rendering hot path.

1. Set the administrator email in `.env`:

   ```text
   ADMIN_EMAILS=admin@example.com
   ```

2. Run `make migrate-up` and restart the service. An existing account with that email is promoted automatically, or you can register it after configuring the environment.
3. Open [http://localhost:8080/admin](http://localhost:8080/admin).

User and project changes are written to `admin_audit_logs`. Disabling a user revokes all sessions immediately, while project changes refresh the runtime cache.

## Production Safety

- Set `HTTP_TRUSTED_PROXIES` to the proxy IP addresses or CIDRs allowed to provide `X-Forwarded-*` headers.
- Session credentials are stored as SHA-256 hashes; migration `021` keeps legacy sessions valid until their normal expiry.
- Redis provides cross-instance authentication and collection rate limits and broadcasts runtime project cache invalidations.
- A PostgreSQL advisory lock ensures only one enabled Worker instance flushes each interval. Set `WORKER_ENABLED=false` where no Worker should run.
- `ANALYTICS_MAX_DAILY_VISITORS` and `ANALYTICS_MAX_DIMENSION_VALUES` bound exact Redis data per project and day. Aggregate totals continue after a bound is reached.

## Roadmap

### Current

- Dynamic SVG counters
- Dynamic SVG badges
- Project dashboard
- Traffic analytics
- User auth and project management

### Next

- Richer SVG widgets
- Trend views and chart surfaces
- Public project dashboards
- Team collaboration support

### Later

- SVG-native comments
- Marketplace and templates
- Self-hosting improvements

## Documentation

- [GETTING_STARTED.md](./GETTING_STARTED.md)
- [API.md](./API.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [DATABASE.md](./DATABASE.md)
- [MIGRATIONS.md](./MIGRATIONS.md)
- [ANALYTICS.md](./ANALYTICS.md)
- [RENDERER.md](./RENDERER.md)
- [REDIS.md](./REDIS.md)
- [INTEGRATION.md](./INTEGRATION.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [AGENT.md](./AGENT.md)

## Contributing

Contributions are welcome.

Before opening a PR, please read:

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [AGENT.md](./AGENT.md)

## License

MIT License.

## Vision

SVGStat is more than a visitor counter.

It is building the infrastructure layer for developer-facing analytics that can be distributed as SVG, cached like static assets, and embedded as easily as an image.
