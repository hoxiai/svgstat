# SVGStat Contracts

> This document defines stable boundaries between APay, SVGStat clients, and
> internal modules. APay owns tenancy, billing, and lifecycle; SVGStat owns
> rendering, analytics, and the runtime projections it serves.

# 1. Contract Classes

SVGStat exposes three distinct contract classes:

* public embed contracts for SVG and the website SDK
* authenticated dashboard and admin contracts
* APay lifecycle synchronization contracts

These contracts must remain separate because they have different latency,
authentication, and failure requirements.

# 2. Public Runtime Contract

Examples:

```text
GET /svg/{projectSlug}/counter/{name}.svg
GET /svg/{projectSlug}/badge/{name}.svg
GET /sdk.js
POST /api/v1/collect
```

Rules:

* never call an external service synchronously
* never write PostgreSQL during SVG rendering
* resolve runtime eligibility from memory or Redis
* validate and bound every public input
* degrade gracefully when analytics storage is unavailable

# 3. Dashboard Contract

Dashboard APIs use authenticated SVGStat sessions and enforce project ownership.

Examples:

```text
GET  /api/v1/projects
GET  /api/v1/projects/{id}/stats
PUT  /api/v1/projects/{id}/website
POST /api/v1/projects/{id}/goals
POST /api/v1/projects/{id}/funnels
```

Every project-scoped operation must resolve the project by both project ID and
authenticated user ID before reading or mutating data.

# 4. Admin Contract

Admin APIs require both an active session and the `admin` role.

State-changing operations must:

* validate a narrow request shape
* commit durable state transactionally
* write an audit record
* revoke sessions when disabling a user
* refresh runtime cache when changing project eligibility

# 5. APay Synchronization Contract

APay uses credentials separate from browser sessions. Every lifecycle event
should carry:

```json
{
  "eventId": "evt_01J...",
  "eventType": "project.plan.updated",
  "occurredAt": "2026-08-24T12:00:00Z",
  "data": {}
}
```

Rules:

* `eventId` is globally unique and safe to retry.
* Requests are authenticated or signature-verified.
* Event type and payload version are explicit.
* SVGStat validates the contract and stores a runtime-ready projection.
* APay remains authoritative for tenancy, billing, and lifecycle.
* Processing outcome is auditable and replayable.

# 6. Error Contract

JSON APIs use a consistent envelope:

```json
{
  "success": false,
  "error": "Human-readable error"
}
```

Expected status codes:

* `400` invalid input
* `401` unauthenticated
* `403` unauthorized or runtime-disabled
* `404` missing resource
* `409` invalid state transition
* `429` rate limited
* `500` internal failure
* `503` temporary dependency failure

# 7. Versioning

Backward-compatible fields may be added without a new API version. Breaking
changes require a new explicit version or a staged migration.

Public SVG URLs and project slugs are compatibility-sensitive and must not be
changed casually.

# 8. Invariants

1. APay owns tenancy, billing, and commercial lifecycle truth.
2. SVGStat owns rendering, analytics, and local runtime truth.
3. Public runtime, dashboard, admin, and integration contracts remain separate.
4. Public rendering never depends on APay or another external service.
5. Authenticated project operations enforce local access.
6. Admin mutations are transactional and auditable.
7. APay synchronization is authenticated and idempotent.
8. Contract changes preserve compatibility or introduce explicit versioning.
