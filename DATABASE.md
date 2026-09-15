# SVGStat Database Design

> PostgreSQL is the durable store for SVGStat runtime configuration and
> analytics. Redis is the hot runtime store. APay remains authoritative for
> tenancy, billing, and commercial lifecycle.

# 1. Ownership

SVGStat persists:

* local dashboard users, roles, sessions, and administration audit logs
* project runtime projections, configuration, and eligibility
* API keys and widget configuration
* conversion goals and funnels
* daily historical analytics
* APay synchronization events

Tenant ownership, subscriptions, orders, invoices, payment records, and the
commercial account lifecycle stay in APay and must not be duplicated here.

# 2. Storage Layers

```text
Process memory: bounded short-lived cache
        │
        ▼
Redis: counters, current analytics, runtime projections
        │
        ▼
PostgreSQL: durable SVGStat state and historical aggregates
```

Rules:

* SVG rendering never writes PostgreSQL.
* Runtime analytics write to Redis first.
* PostgreSQL stores aggregated analytics, not one row per request.
* Redis and process memory must be replaceable projections for lifecycle state.

# 3. Core Relationships

```text
users
  ├── sessions
  ├── projects
  │     ├── api_keys
  │     ├── project_limits
  │     ├── widget_settings
  │     ├── conversion_goals
  │     ├── funnels
  │     └── daily_statistics
  └── admin_audit_logs
```

Each analytics query and runtime key is project-scoped.

# 4. Projects

`projects` is SVGStat's durable runtime representation of a project. It stores
the APay binding, local access relation, slug, runtime status, feature switches,
website tracking configuration, and timestamps.

Rules:

* slugs are globally unique
* project IDs are immutable
* ordinary deletion is soft deletion
* APay lifecycle transitions are accepted only through authenticated,
  idempotent synchronization
* eligibility changes refresh the runtime projection

`external_project_id` and `tenant_id` bind the projection to stable APay
identifiers. They do not make SVGStat the owner of tenant or billing truth.

# 5. Runtime Policy

`project_limits` stores the runtime policy derived from APay plan state.
PostgreSQL is authoritative for the materialized SVGStat projection; Redis and
memory cache the fields needed by public rendering and collection.

`project_sync_events` records authenticated APay synchronization attempts for
idempotency, replay, and operational auditability.

# 6. Historical Analytics

`daily_statistics` stores one aggregate row per project and UTC date. JSONB is
used for bounded distributions such as referrers, paths, devices, events,
segments, funnels, and session quality.

Rules:

* workers upsert on `(project_id, date)`
* high-cardinality dimensions must be bounded
* raw headers, secrets, and complete per-request payloads are not persisted
* retention and future columnar-storage migration should be explicit

# 7. Credentials and Sessions

API keys must be stored as hashes and support scope, expiration, rotation, and
revocation. Browser sessions are separate from integration credentials.

Session cleanup, administrator session policy, and future token hashing are
security responsibilities of SVGStat.

# 8. Administration

State-changing admin operations use database transactions and write
`admin_audit_logs` with actor, action, target, before/after data, IP, and time.

Audit records are append-oriented and must not be modified by ordinary product
flows.

# 9. Transactions and Constraints

Use transactions for operations that must commit atomically, including admin
mutations, session revocation with user disablement, and durable project policy
changes.

Prefer database constraints for uniqueness, foreign keys, enumerated statuses,
and nonnegative limits. Enforce the same rules in the application for clear
errors.

# 10. Invariants

1. APay remains the source of tenancy, billing, and lifecycle truth.
2. PostgreSQL is the durable source of SVGStat runtime and analytics truth.
3. PostgreSQL is never the SVG rendering write path.
4. Runtime analytics always flow through Redis first.
5. Historical analytics remain aggregated.
6. IDs are immutable and public identifiers are compatibility-sensitive.
7. Credentials are hashed and revocable.
8. Redis and memory remain replaceable runtime projections.
9. Schema changes use append-only versioned migrations.
