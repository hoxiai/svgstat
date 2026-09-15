# ANALYTICS.md

# SVGStat Analytics Design

> This document defines the analytics collection pipeline of SVGStat.
>
> Analytics must be high-volume, low-latency, Redis-first, and independent from
> SVG rendering presentation logic.

---

# 1. Purpose

Analytics is responsible for collecting request facts associated with SVG usage.

Core metrics include:

* page views
* unique visitors
* request count
* bot traffic
* referrers
* countries
* browsers
* devices

Analytics is not responsible for:

* rendering SVG
* billing
* authentication UI
* long-running historical reports in hot paths

---

# 2. System Context

SVGStat sits behind APay in the broader SaaS architecture:

* `APay` handles public website entry, pricing, and user portal flows.
* `SVGStat` observes runtime SVG traffic and turns it into analytics.

Therefore analytics data in SVGStat should describe:

* runtime usage of published SVG assets
* project-level embed consumption
* performance-safe operational statistics

It should not become the primary business analytics system for billing or customer management.

---

# 3. Pipeline Overview

Every runtime request should pass through a stable analytics pipeline:

```text
HTTP Request
      │
Normalize request metadata
      │
Resolve project
      │
Bot detection
      │
Visitor identity normalization
      │
Redis pipeline write
      │
Return response
      │
Worker aggregation
      │
PostgreSQL historical storage
```

The response must not wait on historical aggregation.

---

# 4. Request Normalization

Normalize input before counting.

Examples:

* normalize project identifier
* normalize referrer host
* normalize browser family
* normalize device class
* normalize country code
* normalize path and resource name

Normalization rules should be deterministic and consistent across runtime and worker stages.

If labels drift over time, historical reporting becomes noisy and misleading.

---

# 5. Project Resolution

Analytics must resolve the project before writing anything.

Allowed resolution sources:

* in-memory project cache
* Redis project config cache
* durable project config store outside the hot write path

Project resolution must happen before:

* counter updates
* rate limiting
* referrer classification
* dashboard read authorization

Without strict project resolution, multi-tenant isolation breaks.

---

# 6. Bot Detection

Bot detection happens before visitor counting.

Possible signals:

* user-agent rules
* known bot signatures
* CDN or proxy hints
* impossible request patterns

Rules:

* keep hot-path bot checks lightweight
* count bot traffic separately
* do not let bot detection block rendering

Bot traffic can still be useful operationally, but it should not inflate human-facing UV metrics.

---

# 7. Visitor Identity

Unique visitor tracking requires a stable but privacy-conscious identifier.

Possible inputs:

* IP-derived signal
* user-agent-derived signal
* forwarded headers from trusted edge layers
* signed anonymous visitor token when available

Recommended behavior:

* normalize inputs
* hash the final identity
* scope it to project and date bucket when appropriate

Do not store raw personal identifiers when a hashed runtime identity is sufficient.

---

# 8. Metrics Definitions

## PV

Page views count valid render requests.

Every accepted request may increment PV, subject to:

* project validity
* endpoint validity
* optional bot classification rules

---

## UV

Unique visitors count distinct visitor identities within a bucket.

Typical bucket:

* per project
* per day

UV should be derived through Redis set membership or another explicitly documented approximation strategy.

---

## Requests

This is the raw technical request volume.

It may include:

* valid renders
* bots
* cached hits routed through the service

Use it for operational insight, not necessarily for customer-facing "human views".

---

## Dimensions

Dimension distributions include:

* referrer
* country
* browser
* device

These should be normalized into bounded labels before storage.

---

# 9. Redis Write Strategy

Analytics writes must go to Redis first.

Typical operations in one request:

* increment PV
* increment request count
* increment bot count when applicable
* add visitor hash to UV set
* increment referrer hash
* increment country hash
* increment browser hash
* increment device hash

Use pipelining to keep request overhead low.

Do not write per-request rows into PostgreSQL.

---

# 10. Hot Path Constraints

Analytics in request path must avoid:

* PostgreSQL writes
* large scans
* synchronous external API calls
* expensive geo resolution if not cached or bounded
* high-cardinality unbounded labels

The analytics module must be safe to execute on every SVG request.

---

# 11. Historical Aggregation

Workers periodically transform hot Redis data into historical aggregates.

Examples:

* daily PV
* daily UV
* daily referrer distribution
* daily browser distribution
* daily device distribution
* daily country distribution

Historical tables should store aggregated facts only.

Never persist one database row per runtime request.

---

# 12. Dashboard Semantics

Dashboard consumers may include:

* APay user center views
* future public dashboard pages

Dashboard reads may combine:

* Redis for near-real-time today values
* PostgreSQL for yesterday and historical periods

This preserves freshness without making PostgreSQL the hot path.

---

# 13. Failure Handling

Analytics collection should degrade gracefully.

Examples:

* Redis error -> response still returns SVG where possible
* geo resolution unavailable -> record `unknown` country
* bot detection uncertain -> classify conservatively

Preferred failure behavior:

* preserve availability
* keep labels normalized
* record operational error metrics

Perfect analytics is less important than preserving the render path.

---

# 14. Privacy and Data Discipline

Analytics should minimize sensitive data retention.

Rules:

* hash visitor identifiers when possible
* store normalized dimensions, not raw headers
* avoid raw full referrer URLs when host-level data is enough
* avoid embedding secrets or user tokens into analytics payloads

Business identity belongs to APay, not to SVGStat runtime analytics.

---

# 15. Analytics Invariants

The following rules are mandatory:

1. Analytics never renders SVG.
2. Runtime analytics always write to Redis first.
3. Project resolution happens before counting.
4. PostgreSQL stores aggregates, not per-request events.
5. Bot traffic is separated from human-facing metrics.
6. Visitor identity is privacy-conscious and normalized.
7. Analytics failure must not break rendering.

These rules keep SVGStat analytics scalable and operationally safe.
