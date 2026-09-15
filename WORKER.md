# WORKER.md

# SVGStat Worker Design

> This document defines the asynchronous worker subsystem of SVGStat.
>
> Workers move data from hot runtime storage into durable historical storage and
> perform maintenance tasks that must never block user-facing SVG requests.

---

# 1. Purpose

Workers are responsible for asynchronous and maintenance-oriented jobs such as:

* daily aggregation
* historical persistence
* cache warming
* cleanup
* retry and repair

Workers are not responsible for:

* serving live SVG responses
* collecting direct user input
* handling interactive authentication

---

# 2. System Context

In the broader platform:

* `APay` handles the public-facing portal and purchase experience.
* `SVGStat` workers convert runtime analytics into durable SVGStat history and keep runtime caches healthy.

This means worker jobs should focus on SVGStat-owned runtime and historical concerns, not on SaaS billing workflows.

---

# 3. Why Workers Exist

Without workers, the runtime path would be forced to do heavy operations such as:

* historical aggregation
* database writes
* cleanup scans
* cache rebuilds

That would directly harm render latency.

Workers exist to keep:

* request latency low
* runtime logic simple
* storage responsibilities separated

---

# 4. Core Job Types

## Daily Aggregation

Read Redis daily buckets and persist aggregates into PostgreSQL.

Examples:

* daily PV
* daily UV
* referrer distribution
* browser distribution
* country distribution
* device distribution

---

## Cleanup

Remove or expire no-longer-needed runtime keys.

Examples:

* old dedupe markers
* processed UV sets
* temporary locks
* stale project cache snapshots

---

## Cache Warming

Preload hot project configuration or frequently requested templates.

Examples:

* newly provisioned projects from APay
* recently updated widget configs
* popular public counters

---

## Repair and Backfill

Recover from missed windows or operational incidents.

Examples:

* replay aggregation for a skipped day
* repair partial historical rows
* refresh broken cache snapshots

---

# 5. Aggregation Flow

Typical aggregation flow:

```text
Select date bucket
      │
Enumerate project-scoped Redis keys
      │
Read aggregated counters and distributions
      │
Transform into durable schema
      │
Write PostgreSQL transaction
      │
Mark bucket as processed
      │
Expire or archive Redis keys
```

This flow must be safe to retry.

---

# 6. Idempotency

Worker jobs must be idempotent or explicitly deduplicated.

Recommended strategies:

* unique `(project_id, date)` constraint for daily aggregates
* upsert instead of blind insert where appropriate
* checkpoint keys in Redis
* job locks with TTL

Never assume a worker job runs exactly once.

Retries, crashes, and overlapping schedules are normal operational realities.

---

# 7. Scheduling

Workers should be scheduled by task type.

Examples:

* near-real-time lightweight rollups every few minutes
* daily finalize jobs shortly after bucket boundary
* cleanup jobs on a periodic cadence
* cache warm jobs on project sync events

Do not trigger large batch jobs inside request handlers.

---

# 8. Locking

Distributed locks may be needed for:

* daily finalize jobs
* per-project aggregation
* repair or backfill tasks

Lock rules:

* lock scope should be narrow
* locks must have TTL
* lock acquisition failure should defer work instead of blocking the runtime path

SVGStat uses a PostgreSQL advisory lock for the recurring analytics flush. Every API instance may start the Worker, but only the lock holder flushes that interval. Set `WORKER_ENABLED=false` on instances that should never run background work.

---

# 9. Database Writes

Workers are the right place for PostgreSQL writes related to analytics history.

Allowed examples:

* insert daily aggregates
* update historical summaries
* persist precomputed dashboard facts

Workers should write:

* aggregated rows
* bounded payloads
* normalized dimensions

Workers should not write:

* one row per runtime request
* raw unbounded header dumps

---

# 10. Coordination with APay

Workers may respond to lifecycle changes synchronized from APay.

Examples:


* APay plan purchase results in project activation -> refresh project eligibility flags

The authoritative commercial state still comes from APay. Workers only materialize it into runtime-ready form inside SVGStat.

---

# 11. Failure Handling

Worker failure should not break live SVG serving.

Typical degraded outcomes:

* dashboard data is stale
* historical aggregation is delayed
* caches are cooler than ideal

Rules:

* emit metrics and logs
* keep retry paths explicit
* prefer replayable jobs
* avoid partial silent corruption

---

# 12. Observability

Workers should expose metrics such as:

* job duration
* jobs succeeded
* jobs failed
* aggregation lag
* rows written
* keys processed
* retries

Operational visibility matters because many correctness issues surface in workers before they appear in user-facing dashboards.

---

# 13. Operational Guardrails

Avoid these anti-patterns:

* long unbounded scans without buckets
* huge cross-project full rebuilds during peak traffic
* best-effort writes without error accounting
* deleting hot keys before durable persistence succeeds

Prefer:

* bucketed processing
* explicit checkpoints
* narrow retries
* clear ownership between worker, Redis, and PostgreSQL

---

# 14. Worker Invariants

The following rules are mandatory:

1. Workers never block user-facing requests.
2. Workers own asynchronous aggregation and cleanup.
3. Worker jobs are idempotent or explicitly deduplicated.
4. Historical persistence writes aggregated data only.
5. APay lifecycle changes may trigger cache refresh, not hot-path coupling.
6. Failure delays analytics freshness but should not break rendering.

These rules keep SVGStat correct without sacrificing latency.
