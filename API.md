# SVGStat API Design

> SVGStat exposes public runtime APIs, authenticated product APIs, admin APIs,
> and APay integration APIs. These surfaces remain separate.

# 1. Public Runtime

```text
GET  /svg/{projectSlug}/counter/{name}.svg
GET  /svg/{projectSlug}/badge/{name}.svg
GET  /sdk.js
POST /api/v1/collect
```

Requirements:

* no external service dependency
* no PostgreSQL writes during SVG rendering
* project-aware validation and rate limiting
* memory/Redis project resolution
* Redis-first analytics
* graceful degraded SVG response when possible

# 2. Authentication

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

Browser authentication uses an HttpOnly session cookie. State-changing cookie
requests require same-origin CSRF validation. Integration credentials are a
separate concern and must not reuse browser sessions.

# 3. User and Project APIs

Authenticated project routes enforce ownership before accessing project data.

```text
GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/{id}
PUT    /api/v1/projects/{id}
DELETE /api/v1/projects/{id}
```

Project analytics, diagnostics, website tracking, goals, and funnels live under
the same owned project resource.

# 4. Admin APIs

Admin routes require an active SVGStat session and the `admin` role.

```text
GET   /api/v1/admin/overview
GET   /api/v1/admin/users
PATCH /api/v1/admin/users/{id}/status
GET   /api/v1/admin/projects
PATCH /api/v1/admin/projects/{id}/status
PATCH /api/v1/admin/projects/{id}/capabilities
```

Privileged writes are transactional and auditable. Project eligibility changes
refresh runtime cache after commit.

# 5. APay Integration APIs

APay lifecycle synchronization must use authenticated, versioned, idempotent
contracts. SVGStat validates the event and stores a runtime-ready local
projection; APay remains authoritative for tenancy, billing, and lifecycle.

Synchronization failure may delay a business change but never joins the SVG
render path. Payment-provider callbacks terminate in APay rather than in the
SVGStat runtime API.

# 6. Response Envelope

Successful JSON response:

```json
{"success": true, "data": {}}
```

Error response:

```json
{"success": false, "error": "Human-readable error"}
```

# 7. Security

* validate and bound all public inputs
* enforce project ownership on every authenticated project operation
* require admin role for admin routes
* hash persistent credentials
* rate-limit authentication and public collection
* trust proxy headers only from configured proxies
* never log secrets or raw credentials

# 8. Versioning

The `/api/v1` prefix protects authenticated and collection contracts. Public SVG
URLs are compatibility-sensitive. Additive response fields are allowed;
breaking changes require a new version or staged migration.

# 9. Invariants

1. APay is the source of tenancy, billing, and commercial lifecycle truth.
2. SVGStat is the source of rendering, analytics, and local runtime truth.
3. Public runtime APIs stay independent from dashboard, admin, and commerce.
4. SVG rendering never writes PostgreSQL or calls an external service.
5. Authenticated project APIs enforce access to the local project projection.
6. Admin writes are transactional and audited.
7. APay synchronization writes are authenticated and idempotent.
