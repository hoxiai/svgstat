# AGENT.md

# SVGStat Engineering Guide

> This document defines the engineering standards for SVGStat.
>
> Every contributor and every AI coding assistant must follow these rules.
>
> The primary goals are:
>
> * Maintain a clean architecture.
> * Keep the rendering pipeline fast.
> * Preserve long-term maintainability.
> * Build a commercial-grade SaaS platform.

---

# 1. Project Overview

SVGStat is a developer-first SVG Analytics Platform.

Its core capabilities are:

* Dynamic SVG Rendering
* Analytics Collection
* SVG Widgets
* SVG Badges
* README Analytics

SVGStat is **not** a generic analytics platform.

Everything in this repository should strengthen one of these capabilities.

---

# 2. Engineering Philosophy

Every engineering decision should prioritize:

1. Simplicity
2. Performance
3. Maintainability
4. Scalability

Do not introduce abstractions before they are needed.

Do not optimize without measurement.

Do not sacrifice architecture for short-term convenience.

---

# 3. High-Level Architecture

SVGStat is the runtime engine in a two-layer SaaS architecture.

```text
APay (Official Site / User Portal)
        │
        │ purchase and account entry
        ▼
Go SVG Engine
        │
        ├── Renderer
        ├── Analytics
        ├── API
        ├── Worker
        └── Cache
```

Responsibilities are intentionally separated.

APay manages website-facing user entry and account-center experience.

APay manages tenancy, billing, and lifecycle.

Go manages rendering and analytics.

The hot rendering path must not depend on APay availability.

---

# 4. Core Principles

These principles should never be violated.

* Renderer never imports Analytics.
* Analytics never imports Renderer.
* SVG requests never query PostgreSQL directly.
* Redis is always the first write target.
* Business logic never belongs in HTTP handlers.
* SVG output must come from templates.
* Every package owns a single responsibility.
* APay never joins the hot SVG path.

If implementation conflicts with these principles, redesign the implementation.

---

# 5. Repository Structure

```
cmd/
    api/

internal/
    analytics/
    renderer/
    badge/
    counter/
    widget/
    cache/
    middleware/
    worker/
    auth/
    project/
    metrics/
    config/

pkg/

templates/

configs/

scripts/

deploy/

docs/
```

Do not create new top-level directories without architectural discussion.

---

# 6. Package Responsibilities

## analytics

Responsible for:

* Page Views
* Unique Visitors
* Referrer
* Device
* Browser
* Country
* Event Collection

Must never render SVG.

---

## renderer

Responsible for:

* Counter SVG
* Badge SVG
* Widget SVG
* Chart SVG
* Heatmap SVG

Must never:

* Count visitors
* Write Redis
* Query PostgreSQL

---

## worker

Responsible for:

* Aggregation
* Cleanup
* Cache Warming
* Scheduled Jobs

Workers should never block HTTP requests.

---

## cache

Responsible for:

* Memory Cache
* Redis Access
* Cache Strategy

Business logic should not live here.

---

# 7. Dependency Rules

Allowed:

```text
HTTP
 ↓
Service
 ↓
Repository
 ↓
Redis / PostgreSQL
```

Forbidden:

```text
Renderer
 ↓
Analytics
```

Forbidden:

```text
Analytics
 ↓
Renderer
```

Forbidden:

```text
Repository
 ↓
HTTP
```

Dependencies should always point downward.

---

# 8. Go Coding Standards

* Use `context.Context` as the first parameter of request-scoped functions.
* Return errors instead of calling `panic`.
* Prefer constructor injection.
* Keep interfaces small.
* Keep functions focused.
* Avoid global mutable state.
* Avoid reflection unless necessary.

---

# 9. Naming Conventions

Packages:

* singular
* lowercase

Good:

* renderer
* analytics
* cache

Avoid:

* helpers
* utils
* common
* misc

Variables should be descriptive.

Avoid abbreviations that reduce readability.

---

# 10. Redis Rules

Redis is the primary runtime storage.

Analytics writes must go to Redis first.

Typical key format:

```
project:{id}:today
project:{id}:pv
project:{id}:uv
project:{id}:country
project:{id}:browser
project:{id}:referrer
```

Never write directly to PostgreSQL during SVG rendering.

---

# 11. Database Rules

PostgreSQL stores:

* Projects
* Daily Statistics
* Historical Aggregates
* User Configuration

It is not a real-time counter database.

Lifecycle and billing truth stay in APay, not inside SVGStat runtime storage.

Schema changes must be implemented through versioned migrations.

---

# 12. SVG Rendering Rules

Renderer should:

* Load templates
* Apply data
* Produce deterministic SVG

Renderer should never:

* Perform authentication
* Execute business logic
* Access databases directly

Avoid constructing SVG through string concatenation.

Always prefer templates.

---

# 13. API Design

RESTful APIs only.

Rules:

* Stateless
* JSON responses
* Version when necessary
* Consistent error format

Handlers should:

1. Validate input
2. Call services
3. Return responses

Nothing more.

---

# 14. Performance Budget

Target latency:

* P95 < 20 ms
* P99 < 50 ms

SVG requests should avoid:

* Database writes
* Blocking I/O
* Long-running calculations

Prefer:

Memory Cache → Redis → PostgreSQL

---

# 15. Testing

Every package should include tests.

Recommended:

* Unit Tests
* Table-driven Tests
* Benchmark Tests
* Snapshot Tests for SVG

Performance-sensitive code should include benchmarks.

---

# 16. AI Coding Rules

When generating code:

* Follow the existing architecture.
* Reuse existing packages before creating new ones.
* Avoid unnecessary abstractions.
* Do not bypass Redis.
* Do not access PostgreSQL in hot paths.
* Keep modules loosely coupled.
* Preserve single responsibility.
* Do not pull APay business workflows into runtime rendering code.
* Treat local APay lifecycle projections as derived data, not billing or tenant truth.

If multiple designs are possible, choose the one that keeps the architecture simpler.

---

# 17. Code Review Checklist

Before merging:

* Architecture respected.
* No circular dependencies.
* Errors handled.
* Tests updated.
* Documentation updated.
* No duplicated logic.
* No unnecessary allocations.
* Performance impact considered.

Code that merely works is not sufficient.

The implementation should also be maintainable, predictable, and scalable.

---

# 18. Long-Term Vision

SVGStat is intended to become the infrastructure layer for SVG-based analytics.

Every design decision should move the project closer to:

* Better performance
* Better developer experience
* Better scalability
* Better modularity
* Better commercial readiness
