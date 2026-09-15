# REDIS.md

# SVGStat Redis Design

> This document defines how Redis is used inside SVGStat.
>
> Redis is the primary runtime storage layer for analytics and hot counters.
> PostgreSQL remains the durable historical store.

---

# 1. Role of Redis

Redis is the first write target for runtime activity.

It is responsible for:

* hot counters
* today statistics
* unique visitor tracking
* referrer distribution
* browser distribution
* device distribution
* short-lived project runtime state
* queue-friendly aggregation input

Redis is not responsible for:

* long-term billing data
* tenant ownership truth
* append-only audit storage
* historical reporting beyond the hot window

Those belong to APay or PostgreSQL, depending on the domain.

---

# 2. System Context

In the APay + SVGStat architecture:

* `APay` sells plans and exposes the user-facing portal.
* `SVGStat` owns runtime analytics and SVG rendering.

Redis therefore stores runtime facts for SVGStat only.

Do not turn Redis into a cross-system business database shared with APay logic.

---

# 3. Storage Hierarchy

SVGStat follows a strict storage order:

```text
Application Memory
        │
        ▼
      Redis
        │
        ▼
   PostgreSQL
```

Rules:

* read from memory first when possible
* write runtime analytics to Redis first
* aggregate into PostgreSQL asynchronously
* never bypass Redis for hot analytics writes

---

# 4. Key Design Principles

Every Redis key must be:

* project-scoped
* purpose-specific
* easy to expire or aggregate
* safe for multi-tenant isolation

Recommended structure:

```text
svgstat:project:{projectId}:{domain}:{bucket}
```

Examples:

```text
svgstat:project:123:pv:2026-06-26
svgstat:project:123:uv:2026-06-26
svgstat:project:123:country:2026-06-26
svgstat:project:123:browser:2026-06-26
svgstat:project:123:device:2026-06-26
svgstat:project:123:referrer:2026-06-26
svgstat:project:123:counter:downloads
svgstat:project:123:project-config
```

Avoid vague namespaces such as:

* `stats`
* `cache`
* `data`
* `misc`

---

# 5. Recommended Data Structures

## Scalars

Use for:

* page view totals
* request totals
* bot counts
* precomputed numeric counters

Example:

```text
INCR svgstat:project:123:pv:2026-06-26
```

---

## Sets

Use for:

* daily unique visitor membership
* deduplication markers

Example:

```text
SADD svgstat:project:123:uvset:2026-06-26 {visitorHash}
SCARD svgstat:project:123:uvset:2026-06-26
```

If scale requires lower memory usage, a probabilistic structure may be introduced later, but only with explicit accuracy tradeoff documentation.

---

## Hashes

Use for:

* dimension distributions
* grouped metrics
* runtime project metadata snapshots

Example:

```text
HINCRBY svgstat:project:123:country:2026-06-26 US 1
HINCRBY svgstat:project:123:browser:2026-06-26 Chrome 1
```

---

## Sorted Sets

Use only when ranking or ordering is required.

Examples:

* top referrers
* top counters
* ranking-based widgets

Do not use sorted sets when hashes are enough.

---

# 6. Runtime Write Path

Typical analytics write flow:

```text
HTTP Request
    │
Normalize project and request metadata
    │
Detect bot or valid visitor
    │
Pipeline Redis operations
    │
Return SVG
    │
Worker aggregates later
```

Write rules:

* use pipelining for related updates
* keep operations idempotent where practical
* do not block on PostgreSQL
* do not perform large scans in request path

---

# 7. Daily Bucketing

Analytics keys should usually be date-bucketed.

Recommended bucket formats:

* `YYYY-MM-DD` for daily stats
* `YYYY-MM-DD:HH` only when hourly analysis is required

Daily bucketing helps:

* worker aggregation
* TTL management
* backfill logic
* data repair

Do not accumulate all-time mutable analytics in a single unbounded key unless the metric is intentionally global and tiny.

---

# 8. TTL Strategy

Not every Redis key needs the same TTL.

Recommended classes:

## Short-lived runtime keys

Examples:

* request dedupe keys
* temporary session markers
* lock keys

Typical TTL:

* seconds to hours

---

## Daily hot analytics keys

Examples:

* daily PV
* daily UV set
* daily referrer distribution

Typical TTL:

* days to weeks

These should live long enough for workers to aggregate safely and for delayed retries to succeed.

---

## Semi-stable runtime cache

Examples:

* project configuration snapshots
* template metadata
* plan flags

Typical TTL:

* minutes to hours

These keys should also support proactive refresh when APay pushes a project sync event.

---

# 9. Project Configuration Cache

Redis may cache project configuration needed for hot rendering:

* project status
* enabled counters
* public rendering options
* theme or widget configuration
* key rotation metadata

The source of truth for lifecycle remains outside Redis:

* APay for tenancy and subscription state
* PostgreSQL for durable SVGStat project state if persisted locally

Redis stores the hot copy, not the authoritative APay lifecycle record.

---

# 10. Failure Handling

Redis failures must degrade gracefully when possible.

Examples:

* rendering may return cached or default counter values
* analytics increments may be partially skipped rather than blocking response
* dashboard freshness may degrade temporarily

Rules:

* user-visible render availability is more important than perfect instant analytics
* never let a transient Redis error force a blocking PostgreSQL write in hot path
* emit metrics and logs for dropped or deferred analytics

---

# 11. Worker Interaction

Workers read Redis to produce durable aggregates.

Typical worker responsibilities:

* compute daily PV and UV
* persist distributions to PostgreSQL
* archive or expire processed keys
* repair missed windows

Worker rules:

* read well-known key prefixes only
* prefer bounded scans by date bucket
* maintain idempotency

---

# 12. Operational Constraints

Avoid these anti-patterns:

* `KEYS *` in production paths
* cross-project mixed keys
* storing raw personal identifiers
* huge unbounded sets without retention policy
* turning Redis into a document store for arbitrary business payloads

Use:

* explicit prefixes
* pipelining
* bounded scans
* TTL discipline
* hashed or normalized visitor identifiers

---

# 13. Observability

Track Redis behavior with metrics such as:

* pipeline latency
* commands per request
* cache hit ratio
* worker aggregation lag
* expired key counts
* dropped analytics count

Observability is necessary because Redis is a critical runtime dependency, not just a convenience cache.

---

# 14. Redis Invariants

The following rules are mandatory:

1. Redis is always the first write target for runtime analytics.
2. Every key is project-scoped.
3. Hot rendering never depends on PostgreSQL writes.
4. TTL strategy is intentional, not accidental.
5. Worker aggregation consumes Redis buckets asynchronously.
6. Redis does not become the source of truth for SaaS billing or tenant ownership.
7. Runtime data and business data remain separated.
8. Runtime project invalidations are broadcast across instances.
9. Exact visitor and segmentation sets remain within configured cardinality bounds.
10. Authentication and collection rate limits are shared across instances.

These rules protect SVGStat's performance and multi-tenant safety.
