# SVGStat Migration Strategy

> Database migrations are append-only, ordered, reviewable, and aligned with
> SVGStat ownership of runtime configuration, administration, and historical
> analytics, while preserving APay ownership of tenancy, billing, and lifecycle.

# 1. Rules

* never edit an applied migration
* add a new numbered migration for every schema change
* prefer roll-forward correction
* keep migration execution transactional when supported
* preserve public identifiers and backward compatibility

# 2. Safe Changes

Prefer:

* nullable columns
* columns with safe defaults
* new tables
* intentional indexes
* staged backfills
* dual-read or dual-write transitions when necessary

Avoid combining incompatible type changes, destructive drops, and backfills in
one irreversible migration.

# 3. SVGStat Data Ownership

Local users, sessions, roles, project runtime projections, admin audit logs, and
analytics belong to SVGStat. New tables should correspond to an explicit
runtime capability and should not enter the SVG rendering hot path.

Tenancy, subscriptions, orders, invoices, payments, and commercial lifecycle
belong to APay. Integration identifiers and event tables materialize APay state
without transferring that source-of-truth ownership to SVGStat.

# 4. Analytics Discipline

Runtime analytics write Redis first. PostgreSQL migrations define aggregated
historical storage, not per-request event logs.

For JSONB analytics growth:

* bound high-cardinality dimensions
* document defaults and merge semantics
* account for row rewrite cost
* plan a columnar migration before rows become unbounded

# 5. Index Discipline

Add indexes for measured lookup and ordering paths such as:

* unique public identifiers
* project ownership
* lifecycle status
* `(project_id, date)` analytics reads
* expiring sessions and credentials
* audit time and target lookup

Every index has write and storage cost.

# 6. Review Checklist

Before accepting a migration, verify:

* table ownership is explicit
* constraints and foreign keys are correct
* defaults are safe for existing rows
* large-table rewrite risk is understood
* backfill and recovery plans exist
* runtime cache compatibility is preserved
* tests cover the new schema assumptions

# 7. Invariants

1. Migrations are append-only.
2. SVGStat owns its runtime schema; APay owns commercial truth.
3. Historical analytics remain aggregated.
4. Public identifiers remain stable.
5. Runtime projections remain rebuildable from durable state.
6. Destructive changes use staged roll-forward migrations.
