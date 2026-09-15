# ARCHITECTURE.md

# SVGStat System Architecture

> This document defines the architecture of SVGStat.
>
> It describes **why** the system is designed this way, how each component interacts, and which architectural boundaries must never be violated.
>
> If implementation details conflict with this document, the implementation should be reconsidered.

---

# 1. Design Goals

SVGStat is designed around one primary requirement:

> Render dynamic SVGs with extremely low latency while collecting analytics at scale.

The system should satisfy the following goals:

* Low latency
* Horizontal scalability
* Stateless rendering
* Multi-tenant SaaS support
* Clear module boundaries
* Independent deployment
* Long-term maintainability

---

# 2. Architecture Overview

SVGStat is the runtime execution layer behind APay. APay owns the official
website, user portal, tenancy, billing, and commercial lifecycle. SVGStat owns
rendering, analytics, runtime configuration, and the local projections needed
to serve requests without calling APay.

```text
APay (Official Site / User Portal)
        │
        │ authenticated lifecycle sync
        ▼
SVGStat Management API ──────► PostgreSQL
        │                           │
        └──── runtime refresh ──────┤
                                    ▼
Developers / Browsers ─► CDN ─► SVGStat Runtime
                                    │
                         ┌──────────┼──────────┐
                         ▼          ▼          ▼
                     Renderer   Analytics   Worker
                         │          │          │
                         └──────── Redis ◄─────┘
```

The Go service may serve a runtime dashboard and operational admin console, but
those interfaces do not replace APay's user portal or commercial ownership.
APay and PostgreSQL are never queried synchronously by the hot SVG path.

---

# 3. Service Responsibilities

## SVGStat Application

Responsible for:

* SVG rendering
* Analytics collection
* Badge generation
* Widget rendering
* REST API
* Worker execution
* Cache management
* Authentication and sessions
* Runtime project configuration
* Operational admin controls and audit logs

HTTP instances should remain horizontally scalable. Durable SVGStat state and
APay lifecycle projections live in PostgreSQL, hot runtime state lives in
Redis, and process memory is used only as a bounded short-lived cache.

---

## APay

Responsible for:

* official website and purchase entry
* user portal and account-center experience
* tenant ownership
* billing, subscription, and commercial lifecycle truth

APay sends authenticated, versioned, and idempotent lifecycle changes to
SVGStat. SVGStat materializes only the runtime fields it needs. APay never
participates in the hot SVG render path.

---

# 4. Request Flows

## SVG Request

```text
Client
    │
Cloudflare
    │
Go HTTP
    │
Memory Cache
    │
Redis Pipeline
    │
Renderer
    │
SVG
    │
Response
```

Characteristics:

* No PostgreSQL writes
* No external service calls
* No blocking tasks

---

## Dashboard Request

```text
Browser
    │
SVGStat API
    │
Redis
    │
PostgreSQL
    │
JSON Response
```

Dashboard traffic is separated from rendering traffic.

---

## Lifecycle Synchronization

```text
User completes an account or purchase action in APay
    │
    ▼
APay sends an authenticated, idempotent lifecycle event
    │
    ▼
SVGStat validates and stores the local runtime projection
    │
    ▼
SVGStat refreshes runtime cache
```

Synchronization may be retried or delayed without affecting existing render
traffic. Local operational administrators may disable runtime access, but they
do not edit APay billing or tenant truth.

---

# 5. Core Modules

## Renderer

The renderer is responsible only for presentation.

Responsibilities:

* Load SVG templates
* Apply data
* Produce SVG output

The renderer must never:

* Count visitors
* Access PostgreSQL
* Authenticate users

---

## Analytics

Analytics is responsible only for data collection.

Responsibilities:

* Page views
* Unique visitors
* Referrers
* Countries
* Browsers
* Devices

Analytics never generates SVG.

---

## Worker

Workers process asynchronous tasks.

Examples:

* Daily aggregation
* Cache refresh
* Cleanup
* Historical statistics

Workers should never affect request latency.

---

# 6. Storage Strategy

SVGStat uses layered storage.

```text
Application Memory
        │
        ▼
      Redis
        │
        ▼
   PostgreSQL
```

Each layer has a different responsibility.

## Memory

* Project configuration
* Frequently accessed objects
* Small hot datasets

---

## Redis

Runtime data.

Examples:

* Counters
* Today statistics
* Sessions
* Referrers
* Browser distribution

Redis is optimized for write throughput.

---

## PostgreSQL

Persistent storage.

Examples:

* Projects
* Historical reports
* Daily aggregates
* Runtime configuration and lifecycle projections

PostgreSQL is not a real-time analytics database and does not duplicate APay's
billing ledger or tenant ownership model.

---

# 7. Cache Strategy

Cache priority:

```text
Memory
   │
Redis
   │
Database
```

The application should always attempt the highest cache layer first.

---

# 8. Analytics Pipeline

Every request follows the same pipeline.

```text
HTTP Request
      │
Normalize
      │
Bot Detection
      │
Resolve Project
      │
Memory Cache
      │
Redis Pipeline
      │
Response
      │
Worker Aggregation
      │
PostgreSQL
```

Collection should never block rendering.

---

# 9. Rendering Pipeline

Rendering should remain deterministic.

```text
Resolve Project
      │
Load Theme
      │
Load Template
      │
Apply Data
      │
Generate SVG
      │
Compress
      │
Return
```

Rendering must not contain business logic.

---

# 10. Multi-Tenancy

SVGStat is a SaaS platform.

Every request belongs to a project.

Isolation rules:

* Data isolation
* Cache isolation
* Rate limit isolation
* API key isolation

No project should be able to access another project's data.

---

# 11. Horizontal Scaling

The Go service is stateless.

Any instance should handle any request.

```text
Cloudflare
      │
Load Balancer
      │
 ┌────┴────┐
 │         │
Go #1   Go #2
 │         │
 └────┬────┘
      ▼
Redis Cluster
      ▼
PostgreSQL
```

Scaling should require adding instances, not changing code.

---

# 12. Failure Handling

Failures should degrade gracefully.

Examples:

* Redis unavailable → return SVG with cached/default values if possible.
* Worker unavailable → analytics aggregation delayed, rendering unaffected.
* PostgreSQL unavailable → rendering continues using cached runtime data where feasible.

The rendering path should be resilient.

Multi-instance rules:

* runtime project writes publish Redis invalidation messages so every instance drops stale process-memory entries
* analytics workers use a PostgreSQL advisory lock so only one instance flushes a given interval
* authentication and collection limits use Redis for cross-instance consistency
* Redis failures may fall back to bounded local rate limiting, but never disable validation or authorization

---

# 13. Security

Never trust client input.

All requests should be:

* validated
* sanitized
* rate-limited

Secrets must never be embedded in SVG output.

Forwarded client and scheme headers are accepted only from explicitly configured trusted proxy IPs or CIDRs. Browser session tokens are stored as hashes.

Operational endpoints are separated by purpose:

* `/health` reports process liveness
* `/ready` verifies PostgreSQL and Redis readiness
* `/metrics` exposes runtime counters in Prometheus text format

Exact visitor and segmentation sets use configurable per-project daily bounds. When a bound is reached, aggregate traffic totals continue while new exact visitor detail and high-cardinality dimensions are dropped.

---

# 14. Future Expansion

New features should extend existing modules rather than rewrite them.

Potential additions:

* Comment SVG
* Timeline widgets
* Heatmaps
* Public dashboards
* Team analytics
* Plugin system

Architecture should remain stable as capabilities grow.

---

# 15. Architectural Invariants

The following rules are non-negotiable:

1. Renderer never imports Analytics.
2. Analytics never imports Renderer.
3. SVG requests never write to PostgreSQL.
4. Runtime analytics always go through Redis first.
5. Business logic never lives in HTTP handlers.
6. Workers never block user requests.
7. Templates generate SVG; code should not manually concatenate SVG strings.
8. Every package owns a single responsibility.
9. Dashboard, administration, APay, and billing never join the hot SVG path.
10. Services communicate through stable APIs, not shared implementation details.
11. Maintainability is more important than cleverness.

These invariants define the long-term architecture of SVGStat and should guide every future design decision.
