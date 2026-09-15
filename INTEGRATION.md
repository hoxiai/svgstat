# SVGStat APay Integration

> This document defines how APay synchronizes commercial lifecycle state into
> SVGStat without entering the SVG rendering hot path.

# 1. Ownership

APay is the authoritative owner of:

* official website and user portal
* tenancy and account-center identity
* billing, subscriptions, and purchase history
* commercial project lifecycle and plan entitlements

SVGStat is the authoritative owner of:

* rendering configuration
* runtime and historical analytics
* local runtime projections and cache state
* operational administration and audit history

Local dashboard identities and project access records support SVGStat operation;
they do not replace APay's tenant, billing, or lifecycle records.

# 2. Integration Principle

APay uses explicit, authenticated APIs or verified webhooks. It never
participates synchronously in SVG rendering.

```text
APay lifecycle event
        │
        ▼
Authenticated SVGStat API
        │
        ▼
PostgreSQL runtime projection
        │
        ▼
Runtime cache refresh
```

The render flow remains independent:

```text
Embed request -> memory cache -> Redis -> renderer -> SVG response
```

# 3. Purchase and Lifecycle Flow

A purchase flow completes in APay. APay then delivers the resulting entitlement
or lifecycle change through a verified and idempotent callback.

```text
User completes purchase in APay
        │
        ▼
APay commits billing and lifecycle truth
        │
        ▼
APay sends a signed synchronization event
        │
        ▼
SVGStat updates its local runtime projection
        │
        ▼
SVGStat refreshes runtime eligibility cache
```

Rules:

* APay persists the authoritative commercial result.
* SVGStat persists only the fields required for runtime decisions.
* Synchronization retries must be idempotent.
* APay availability must not affect existing SVG requests.
* Raw payment payloads and secrets must not enter analytics keys.

# 4. Authentication

APay integration credentials must be separate from browser sessions.

Recommended controls:

* hashed API keys or signed requests
* explicit scopes
* expiration and rotation
* request timestamps and replay protection
* per-client rate limits
* audit records for state-changing operations

Never expose internal integration credentials in browser code or SVG URLs.

# 5. Failure Handling

APay synchronization failure may delay a lifecycle or entitlement change. It
must not break existing render traffic.

State-changing callbacks should support:

* stable event IDs
* safe retries
* explicit accepted, processed, rejected, and failed states
* operational visibility and replay

# 6. Invariants

1. APay owns tenancy, billing, and commercial lifecycle truth.
2. SVGStat owns rendering, analytics, and local runtime truth.
3. APay communicates through authenticated contracts.
4. APay calls never join the SVG hot path.
5. State-changing callbacks are verified, idempotent, and auditable.
6. PostgreSQL stores durable local projections; Redis stores hot copies.
7. Integration outages degrade change freshness, not existing rendering.
