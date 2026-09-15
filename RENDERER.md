# RENDERER.md

# SVGStat Renderer Design

> This document defines the SVG rendering subsystem of SVGStat.
>
> The renderer is presentation-only. It transforms project configuration and
> metric inputs into deterministic SVG output.

---

# 1. Purpose

The renderer is responsible for:

* counter SVG
* badge SVG
* widget SVG
* chart SVG
* heatmap SVG in future versions

The renderer is not responsible for:

* analytics counting
* billing
* project provisioning
* user authentication
* direct database writes

---

# 2. System Position

In the larger platform:

* `APay` is the public website and user portal.
* `SVGStat` serves the final embed URL that developers place into README files, Markdown, and websites.

The renderer is the final step of that runtime chain.

Only SVGStat, should participate synchronously once a hot SVG request reaches the renderer.

---

# 3. Rendering Goals

The renderer should optimize for:

* deterministic output
* low latency
* stable templates
* easy cacheability
* low memory churn
* predictable response size

Target characteristics:

* no blocking business logic
* no PostgreSQL writes
* no cross-service dependency in hot path

---

# 4. Rendering Pipeline

Recommended rendering flow:

```text
Resolve project
      │
Load project render config
      │
Resolve render type
      │
Load template
      │
Apply normalized data
      │
Generate SVG
      │
Optional minify or compress
      │
Return response
```

This flow should stay simple and deterministic.

---

# 5. Inputs

The renderer may consume:

* project configuration
* theme or widget configuration
* counter values
* chart data that has already been normalized
* presentation options such as color, label, width, style, or format

The renderer should not own the logic that decides whether a visitor is unique or whether a plan is billable.

Those decisions belong elsewhere.

---

# 6. Template Strategy

SVG should be generated from templates, not from large string concatenation blocks.

Templates provide:

* consistency
* composability
* easier testing
* easier visual iteration

Recommended template rules:

* one template owns one visual concern
* dynamic values are injected through a narrow data model
* templates remain presentation-focused

Avoid mixing:

* project lifecycle rules
* analytics collection
* HTTP request parsing

inside template rendering code.

---

# 7. Render Types

## Counter

Simple numeric presentation.

Examples:

* visitors
* downloads
* stars
* custom count

Counter rendering should remain extremely cheap.

---

## Badge

Compact status-like presentation.

Examples:

* visitors badge
* build status badge
* plan badge
* version badge

Badges should favor small payloads and strong cacheability.

---

## Widget

Richer layouts for Markdown dashboards.

Examples:

* overview cards
* summary panels
* mini charts
* project snapshots

Widgets may have slightly heavier render cost than counters, but must still respect hot-path limits.

---

## Chart

Structured SVG chart output from precomputed or normalized datasets.

Examples:

* trends
* heatmaps
* timelines

The renderer should draw charts, not compute analytics semantics from raw events.

---

# 8. Data Source Rules

Hot render inputs should come from:

* memory cache
* Redis
* lightweight precomputed values

Hot render inputs should not come from:

* synchronous APay calls
* PostgreSQL writes
* large historical scans

If a render type depends on heavy history, that history should be pre-aggregated by workers first.

---

# 9. Response Rules

Renderer output must be:

* valid SVG
* deterministic for the same inputs
* safe to embed
* free of secrets

Recommended headers:

* `Content-Type: image/svg+xml`
* cache headers appropriate to freshness requirements
* compression when supported and beneficial

Never embed:

* secrets
* raw access tokens
* APay internal identifiers unless explicitly safe

---

# 10. Caching

The renderer should benefit from layered caching.

Examples:

* memory cache for hot project config
* Redis for hot counters
* CDN cache for stable SVG URLs

Cache design should respect correctness:

* short TTL for rapidly changing counters
* longer TTL for static badges or theme assets
* explicit cache busting when APay updates project configuration

---

# 11. Failure Handling

Rendering should degrade gracefully.

Examples:

* missing live metric -> render fallback value
* Redis unavailable -> render last known or default value when possible
* unknown theme option -> use default template variant

User-visible rendering continuity matters more than perfect freshness in every failure case.

---

# 12. Security

All renderer inputs must be validated and sanitized.

Watch for:

* untrusted label text
* malformed color values
* oversized dimensions
* injection into SVG attributes or text nodes

The renderer must treat all incoming request parameters as untrusted.

---

# 13. Performance Discipline

Rendering should avoid:

* repeated template parsing per request if preload or reuse is possible
* heavy allocations for tiny counters
* large branching trees for simple badge variants
* expensive text measurement in hot path unless cached

Benchmark performance-sensitive renderers.

Counter and badge rendering should remain the cheapest workloads in the service.

---

# 14. Renderer Invariants

The following rules are mandatory:

1. Renderer never imports analytics logic.
2. Renderer never writes PostgreSQL.
3. Renderer uses templates instead of ad hoc SVG string building.
4. Renderer consumes normalized inputs only.
5. Renderer never depends on APay in hot path.
6. Renderer output is deterministic and safe to embed.
7. Rendering failure should degrade gracefully where possible.

These rules keep SVGStat rendering fast, modular, and predictable.
