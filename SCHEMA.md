# SVGStat Schema

> This document describes the logical schema implemented by migrations. The
> migration files remain the executable source for exact columns and constraints.

# 1. Product Tables

## users

Stores the local operational identity used by SVGStat dashboard and admin
interfaces. It does not replace APay tenant or billing identity.

## sessions

Owns authenticated browser sessions. Sessions belong to users and are revoked
when a user is disabled.

## projects

Stores the local project access relation, APay binding, slug, runtime status,
rendering capabilities, website analytics configuration, and eligibility.

Fields such as `external_project_id` and `tenant_id` bind the runtime projection
to stable APay identities; they do not duplicate APay ownership truth.

## admin_audit_logs

Append-oriented record of privileged mutations.

# 2. Runtime Configuration

## api_keys

Hashed, scoped, expiring, and revocable project credentials.

## widget_settings

Versioned JSONB presentation configuration keyed by project and widget.

## project_limits

Runtime policy derived from APay plan state, such as request limits, cache TTL,
and feature eligibility.

## project_sync_events

Authenticated APay synchronization events. The table provides stable event
identity, status, payload auditability, retry, and replay semantics.

# 3. Analytics

## daily_statistics

One row per `(project_id, UTC date)` containing totals and bounded JSONB
distributions for traffic, attribution, events, conversion, segments, and
session quality.

The worker repeatedly upserts today and yesterday from Redis. No per-request
analytics table is required for the current privacy-conscious aggregate model.

## conversion_goals

Project-scoped named conversion events.

## funnels

Project-scoped ordered event steps stored as a validated JSONB array.

# 4. Identifier Rules

* public project slugs are unique among non-deleted projects
* project and user IDs are immutable strings
* integration event IDs are globally unique and retry-safe
* credentials store hashes, never plaintext
* foreign-key relationships are project-scoped

# 5. Lifecycle Rules

Project statuses are:

```text
pending | active | grace | disabled | archived
```

User statuses are:

```text
active | disabled
```

User roles are:

```text
user | admin
```

APay lifecycle changes must be synchronized into the local project projection.
After the durable transaction commits, SVGStat refreshes affected runtime cache
records. Local administrative disables are operational overrides, not billing
or tenant lifecycle changes.

# 6. JSONB Rules

Use JSONB for bounded, evolving aggregate distributions and presentation
settings. Do not use JSONB for primary identity, ownership, commonly filtered
status, or core relationships.

# 7. Growth Path

Teams, subscriptions, orders, invoices, and payment records remain in APay. If
analytics cardinality or cross-project querying outgrows daily JSONB rows, move
analytical facts to a columnar store while PostgreSQL keeps SVGStat runtime
truth.

# 8. Invariants

1. APay owns tenancy, billing, and commercial lifecycle truth.
2. SVGStat owns rendering, analytics, and its local runtime configuration.
3. Runtime caches never override PostgreSQL projections or APay lifecycle truth.
4. Historical analytics are aggregated and project-scoped.
5. Public and integration identifiers remain stable.
6. Audit and integration event records preserve traceability.
7. Schema evolution occurs through append-only migrations.
