# Sunkern — Design Notes

## 1. Why Sunkern Exists

### The Person Behind It

Irving Dinh, software engineer based in Ho Chi Minh City, Vietnam. Works at enterprise companies (Grab, Axon, SAP) by day, builds side projects every day. Used to explore a new tech stack with each project, but has shifted focus: now it's about testing product ideas fast, not learning new frameworks.

### The Workflow

Irving uses Claude Code in a fully autonomous loop — session after session — to go from a product spec to a deployed application. The AI agent sets up the repository, implements features, tests them (opens a real browser with `@playwright/cli`, hits APIs with `curl`), and deploys. One prompt, one deployed product.

### The Pain (After ~10 AI-Built Projects)

1. **Repetitive stack specification.** Every project required re-specifying the full technical stack from frontend to backend. Without strong guidance, the AI agent drifts into implementation patterns Irving doesn't prefer. Clunky and ineffective.

2. **Deployment and operations friction.** Every project had a different stack, meaning different build processes, different deployment methods, different operational patterns. No consistency, too much friction.

### The Solution

Sunkern is a **template repository** — a single, opinionated technical stack that the AI agent forks to start each new project. It provides:

- Irving's coding style and mental model, enforced by the framework
- Zero 3rd-party dependencies (the framework IS the dependency)
- A truly standalone application: one binary, one Dockerfile, one data directory
- Built-in admin app with everything operational (auth, user management, logs, metrics, uploads, notifications, settings)
- Comprehensive documentation (CLAUDE.md + Claude Code skill) so the AI agent needs zero additional specification — just a product spec

**The promise:** fork Sunkern, give the AI agent a product spec, get a deployed product. No tech stack decisions, no architectural guidance, no deployment instructions.

---

## 2. Architecture Overview

```
sunkern/
  framework/    # Go — low-level lib, enforces mental model, super opinionated
  service/      # Go — the actual product service, built on framework
  admin/        # (later) Vite + React + React Router — admin dashboard
  ui/           # (later) Vite + React + React Router — user-facing app
```

### The Framework (`./framework`)

A low-level Go library that makes the service itself look less complex. Super opinionated — it enforces Irving's mental model on everything. It includes all operational concerns so that no 3rd-party infrastructure is ever needed.

**Rationale:** If something is (1) opinionated by Irving's mental model, or (2) used by more than one module, it belongs in the framework. Everything else belongs in the service.

Examples:
- Base model struct (timestamps, IDs, soft deletes) → framework
- Migration engine, ORM, SQL builder → framework
- Auth primitives (hashing, JWT, token utilities) → framework
- In-memory cache, event bus, cron scheduler, job queue → framework
- Domain-specific models, business logic → service

### The Service (`./service`)

The actual product, built on top of the framework. Ships with built-in modules (admin, auth, notifications) that every project needs. The AI agent extends it with domain-specific modules.

- Runs on port **19110** by default
- On build: embeds admin-ui and user-facing-ui artifacts into the binary
- Routing:
    - `/admin` → admin-ui built artifacts
    - `/api` → service API handlers
    - `/*` → user-facing ui

### Admin UI (`./admin`) — Later

- Vite + React.js + React Router
- Port **19105** in development
- Vite proxy: `/api*` → `localhost:19110` (no CORS needed in dev)

### User-facing UI (`./ui`) — Later

- Vite + React.js + React Router
- Port **19100** in development
- Vite proxy: `/api*` → `localhost:19110` (no CORS needed in dev)
- Sunkern ships nothing here — the AI agent builds the entire user-facing app per-project
- SSR/SEO is explicitly out of scope (accepted trade-off)

---

## 3. Standalone Philosophy

Every design decision serves the standalone nature of the application.

### Single Binary Deployment

The Go service compiles to a single binary. During build, admin-ui and user-facing-ui artifacts are embedded into the binary via Go's `embed` package. One binary = one deployment artifact.

### Data Directory

All runtime and stateful data lives under a single directory.

- Default: `~/.standalone`
- Configurable via config
- Contains: database, uploads, logs, config file

```
{DATA_DIR}/
  config.json         # application config (optional, can use env vars instead)
  database.sqlite     # the one and only database
  uploads/            # file uploads
    {YYYY}/{MM}/{DD}/{nanoid}/{filename}
  logs/
    YYYY_MM_DD.log    # daily JSONL log files
```

### No External Infrastructure

| Instead of     | Sunkern uses                   |
|----------------|--------------------------------|
| PostgreSQL     | SQLite (CGo, C driver)         |
| Redis          | In-memory cache (TTL + LRU)    |
| Kafka          | In-process reactive event bus  |
| RabbitMQ       | SQLite-backed job queue        |
| Crontab        | SQLite-backed cron scheduler   |
| Sentry         | Built-in error tracker (TBD)   |
| S3 (default)   | Local filesystem uploads       |
| SMTP           | Resend API (when configured)   |

### Docker Target

- Multi-stage Dockerfile: stage 1 with C toolchain for CGo compilation, stage 2 minimal final image
- Target: **under 15MB** Docker image
- Deployable to Fly.io, Railway, Cloud Run

### Always Latest Go

The project always uses the latest Go version. No backward compatibility concerns.

---

## 4. Module System

Both the framework and service strictly follow modular design.

### Framework Modules — Flat, Package-Separated

```
framework/
  app/          # application lifecycle, IoC container
  http/         # HTTP server, router, middleware
  db/           # SQLite, migrations, ORM/SQL builder
  cache/        # in-memory cache
  event/        # reactive event bus
  cron/         # cron scheduler
  queue/        # job queue
  ...           # etc.
```

No deep nesting. Each package is one concern.

### Service Modules — Feature-Based, Hierarchical

> Note: file tree below is rough thinking, needs research for Go best practices later.

```
service/
  migrations/                     # centralized, ordered (NOT per-module)
  features/
    adminmod/                     # admin module (BUILT-IN)
      auth/                       # admin authentication
      usermgmt/                   # user management
      logmgmt/                    # log viewer
      notificationmgmt/          # notification management
      uploadmgmt/                # upload management
      settingsmgmt/              # settings management
      ...
    authmod/                      # user-facing auth (BUILT-IN)
    notificationmod/              # user-facing notifications (BUILT-IN)
```

Key patterns:
- `*mod` = top-level feature module
- `adminmod/*mgmt` = admin sub-module for managing a domain
- Modules can have sub-modules, recursively, unlimited nesting
- Each module/sub-module is self-contained: APIs, cron, jobs, workers, logic — all colocated

### Frontend Modules — Mirroring Backend

```
admin/ (or ui/)
  src/
    main.tsx
    routes.tsx
    lib/                          # shared utilities
    features/
      core/                       # shared components, pages, hooks
      auth/
      ...
```

React Query for data fetching across both admin and user-facing apps.

### What the AI Agent Adds (Example: StackOverflow-for-AI)

After forking Sunkern, the AI would add:

**Service:**
- `service/migrations/` — new migration files on top of built-in ones
- `service/features/adminmod/questionmgmt/` — admin management for questions
- `service/features/adminmod/agentmgmt/` — admin management for AI agents
- `service/features/questionmod/` — user-facing question logic
- `service/features/agentmod/` — AI agent answering logic

**Admin:** new pages/components under `admin/src/features/`

**UI:** entire user-facing app built from scratch under `ui/src/features/`

---

## 5. Dependency Injection & IoC Container

### Decision: In-House, Zero Dependencies

Currently using `uber-go/fx` for experimentation only. Will be replaced with a hand-rolled in-house library — just enough functionality, no more. Consistent with the zero 3rd-party dependency philosophy.

### Inspiration: Laravel Facade + IoC Container

**IoC (Inversion of Control) container:**
- Manages singleton instances of both framework-level and service-level components
- DB, cache, logger, event bus, config, and module-specific services all live in the same container
- Modules register their own services into the container during boot

**Facade pattern:**
- Provides static-like access to container-resolved services from anywhere
- Any code, in any module, can resolve what it needs through the facade
- Avoids deep dependency passing — practical for AI-generated code

### Module Composition

Inspired by `fx.Module` composition:
- Module → Sub-module (recursive, unlimited depth) → Capabilities (http, cron, worker, service, etc.)
- Each module declares what it provides
- Framework manages lifecycle and wiring

---

## 6. Database Layer

### SQLite via CGo

- CGo build with the C SQLite library — not a pure-Go driver
- **Rationale:** dedicated and optimized for this use case; CGo gives access to the full SQLite C API, enabling fine-tuned performance (WAL mode, custom pragmas, etc.)
- Single database file: `{DATA_DIR}/database.sqlite`

### Migrations

- Framework provides the migration engine (inside the `db` or `sqlite` package)
- Migrations are **centralized** in `./service/migrations/` — not split per module
    - **Rationale:** cross-module schema concerns need a single, ordered sequence. Splitting by module would create ordering ambiguity and cross-module reference issues.
- Sunkern ships built-in migrations (users table, settings table, etc.)
- AI agent adds new migration files on top of the existing ones
- **Auto-run on application start** — no manual migration step, ever

### In-House ORM / SQL Builder

- Build our own, dedicated and optimized for SQLite
- SQL builder style — not raw SQL strings, not a heavy ORM like GORM
- Tightly integrated with the framework — not designed as a generic, reusable library
- **Rationale:** generic ORMs add complexity and dependencies for features we'll never use. A purpose-built tool can be simpler and faster for our exact use case.

### Base Model

The framework provides a base model struct with common fields:
- ID generation
- Timestamps (created_at, updated_at)
- Soft deletes (deleted_at)

Every domain model in the service builds on top of this.

---

## 7. HTTP Layer

### Router

- Built on top of Go's `net/http` (Go 1.22+ `http.ServeMux` with pattern matching)
- If Gin-like ergonomics are needed (route groups, parameter binding, response helpers, etc.), build them in-house
- **Rationale:** `net/http` in modern Go is capable enough. Building on it keeps dependencies at zero and gives full control.

### API Style

- Best practice, standardized — **needs deep research**
- Research context: APIs are consumed by React apps using **React Query**
- Research should cover: response envelope format, pagination strategy (cursor vs offset), error response format, React Query cache-key friendliness, predictable response shapes

### Middleware Stack (Baseline)

- Authentication
- Logging (request/response)
- Request ID generation
- Rate limiting
- CORS: **disabled by default** (single-domain, everything behind one binary)
    - Can be enabled when the backend serves a mobile app or external API consumers

---

## 8. Config vs Settings

Two distinct systems for application parameters. This distinction is fundamental to the architecture.

### Config — Static, Boot-Time

- Set before or at application start, **cannot change at runtime**
- Resolution order (highest priority first):
    1. Environment variables
    2. `{DATA_DIR}/config.json`
    3. Defaults (hardcoded in the framework)
- Env var naming: **flat, no prefix** (e.g., `PORT`, `DATA_DIR`, `JWT_SECRET` — not `SUNKERN_PORT`)
- Contains everything static: port, data dir path, log level, DB pragmas, secrets (JWT secret, S3 credentials, Resend API token, etc.)

**Rationale for flat env vars:** these are standalone apps, one per container. No namespace collision risk. Simpler for the AI agent and for platform env var configuration.

### Settings — Dynamic, Runtime

- Stored in the **database**
- Editable at runtime via the **admin dashboard**
- Examples: storage backend (local vs S3), email sending enabled/disabled, feature flags, notification preferences
- Changes take effect immediately, no restart required

**Rationale for the split:** config is infrastructure (where to listen, what secrets to use), settings are behavior (should emails be sent, where to store uploads). Infrastructure is fixed per deployment; behavior is adjusted by operators.

---

## 9. Operational Built-ins

> Philosophy: like Laravel/Nest.js — everything is provided out of the box. The developer (or AI agent) just picks it up and uses it. All operational built-ins are observable and controllable through the admin dashboard. Zero external monitoring or management tools needed.

### Cache

- In-memory, **no persistence guarantee** (lost on restart — by design)
- TTL-based expiration + LRU eviction
- Standard key-value cache, nothing exotic
- Cache stats visible in admin dashboard

### Event System

- In-process, reactive event bus — **not** full event-sourcing (no append-only log, no replay)
- Inspired by Nest.js event system
- Key use case: background worker completes work → emits event → SSE endpoint pushes update to specific user
- Enables the pattern: async processing + real-time UI updates
- Event flow visible in admin dashboard

### Cron Jobs

- Definitions stored in SQLite
- Framework provides the scheduler; modules just register cron definitions
- Admin can view schedules, trigger manually, enable/disable
- Built-in crons: daily log cleanup (delete logs older than 7 days), notification digest

### Queue / Worker Jobs

- Job records stored in SQLite
- Full lifecycle: enqueue → pick up → execute → complete/fail → retry → dead-letter
- Admin can view all queued/running/failed jobs, see metrics, retry or discard

### Admin Visibility — Core Value Proposition

**Every operational built-in has a corresponding admin sub-module.** Cache stats, event flow, cron schedules, job queues, logs, errors — all visible and controllable from the admin dashboard without any external tool.

---

## 10. Logging

Opinionated, dual-output strategy:

1. **Console (stdout):** JSON serialized — for container runtime log collection
2. **File:** JSONL serialized to `{DATA_DIR}/logs/YYYY_MM_DD.log` — for the admin log viewer

The admin dashboard provides a built-in log viewer that can read and filter these log files.

A **built-in daily cron job** deletes log files older than 7 days.

**Rationale:** JSON for machine parsing (container platforms), JSONL files for the admin viewer (no external log aggregation needed). 7-day retention keeps the data dir manageable for a standalone app.

---

## 11. Email

### Default: Write-Only (No Sending)

Every email the application "sends" is **always written to a database table** — this is the audit/history record and is non-negotiable.

Emails are **NOT actually sent** until two conditions are met:
1. Email sending is **enabled** via a setting (runtime, admin dashboard)
2. A **Resend API token** is provided in config

**Email provider: Resend** (API-based, no SMTP complexity).

### Why This Design

- In development or early stages, you can see every "sent" email in the admin dashboard without configuring any email service
- Prevents accidental email sends from dev/staging environments
- Provides a complete email audit trail regardless of whether sending is enabled
- Resend is API-based (single HTTP call), no SMTP server management

---

## 12. Notifications

### User/Admin Preferences

- Each user and admin has notification preferences (stored as part of their profile)
- Default: **in-app only**
- Opt-in: email notifications (requires email to be configured)

### Sending Notifications (Two Methods)

1. **Direct function call** — any module can call the notification service directly
2. **Event bus** — modules emit events, notification module listens and routes accordingly

### Delivery Options (Per Notification)

Each notification call specifies:
- **Email?** Default: no (respects user preference). Can override to also send email.
- **Batched or immediate?** Default: batched — a daily cron job collects notifications and sends a digest email. Override: send immediately.

### Viewing Notifications

- **User-facing:** API-only (unread count, list all, mark as read)
- **Admin dashboard:** full notification management UI (view all notifications, filter by user, etc.)

---

## 13. Authentication & Authorization

### Framework Provides (Primitives)

- Password hashing (bcrypt or argon2)
- JWT generation and validation
- Token utilities
- Auth middleware helpers

### Admin Auth (`adminmod/auth`) — Built-in

- **Multiple admins** supported
- **Two roles:** Super Admin (can manage other admins) and Admin
- Email + password authentication (initial implementation)
- Future: social login via Firebase
- Stateless JWT auth with long-lived session support (likely: short-lived access token + long-lived refresh token)
- **API tokens** — admins can generate tokens for programmatic access

### User Auth (`authmod`) — Built-in

- Built-in **"User" role** by default
- **Soft-delete** support — admin can deactivate/activate users (block abusive users without losing data)
- Email + password authentication (initial implementation)
- Future: social login via Firebase
- Stateless JWT auth with long-lived session support
- **API tokens** — users can generate tokens for programmatic access

### Shared Patterns

- Stateless JWT: short-lived access token + long-lived refresh token (keeps users logged in)
- API token system for both admin and user programmatic access
- Firebase social login deferred — not built-in day 1

---

## 14. File Uploads & Storage

### Upload Path Convention

```
{DATA_DIR}/uploads/{YYYY}/{MM}/{DD}/{nanoid}/{filename}
```

- **nanoid** for uniqueness — no filename collisions
- Auto-generated **thumbnail** for image uploads
- Database tracks **who uploaded** each file (ownership/audit)

### Storage Backends

- **Default: local filesystem** (under `DATA_DIR/uploads/`)
- **Optional: S3-compatible bucket** — same path structure, enabled via a **setting** (runtime, admin dashboard)
- Switchable at runtime without restart, without changing the path convention

**Rationale:** local filesystem is zero-config and works everywhere. S3 is there for when the app scales or when the deployment platform doesn't support persistent volumes.

---

## 15. Error Handling — Needs Deep Research

Three areas to research:

1. **Standard error type** — the framework should enforce a standard error type that all modules use. Exact design TBD (research Go best practices in standalone/API context).
2. **API error responses** — standardized error response format for API consumers. Research alongside the broader API style research.
3. **Built-in error tracker (Sentry-like)** — capture panics and errors, surface them in the admin dashboard. Exploratory idea — feasibility TBD but worth pursuing.

---

## 16. AI Agent Experience

This is the key deliverable. Everything in Sunkern exists to make this workflow possible.

### Two Layers of Documentation

1. **`CLAUDE.md`** — concise, lives in the repo root
    - Instructs the agent to load the Sunkern Claude Code skill
    - Contains a "Manual Testing" section
    - Minimal — the skill is the real payload

2. **Sunkern Claude Code skill** — super detailed, like Laravel or Nest.js entire documentation
    - All conventions: how to add modules, sub-modules, migrations, routes, cron, workers
    - How to extend the admin app
    - How to build the user-facing app from scratch
    - Combined with the in-repo framework code as living reference, the AI agent has complete context

### Additional Skills (Later)

- Deployment skills: Fly.io, Railway, Cloud Run — separate skills added later
- Other tooling skills as needed

### The Code as Documentation

The code structure itself is conventional enough that the AI can pattern-match from existing modules. The documentation and the code reinforce each other.

---

## 17. Testing Strategy

### No Unit Tests. No Test Framework. Real Integration Testing Only.

- **Backend:** `curl` against the running service
- **Frontend:** `@playwright/cli` to open a real browser and test
- **Spin up:** `make dev` (or similar) — trivial to start the whole application

### The Standalone Testing Advantage

Because everything is in one binary with one data directory, the AI agent can:
- **Inspect** `DATA_DIR` directly (read logs, check uploads)
- **Query** the SQLite database directly (verify data state)
- **Nuke** everything (`rm -rf ~/.standalone`) for a guaranteed clean test state
- **Import** test data for specific scenarios

All of this is documented in the "Manual Testing" section of `CLAUDE.md`.

**Rationale:** for standalone apps built by an AI agent, real integration tests against the actual running application are more valuable than unit tests. The AI can directly observe the system's state, making debugging trivial. No test infrastructure to maintain.

---

## 18. Build & Deployment — Current Scope

### What's In Scope Now

1. Multi-stage Dockerfile (CGo build stage + minimal final image)
2. `docker build` succeeds
3. `docker run` starts the application correctly
4. Playwright/curl manual testing confirms the container behaves like dev

### What's Deferred

- Platform-specific deployment (Fly.io, Railway, Cloud Run) — will be added as separate skills later
- Volume mount configuration per platform
- CI/CD pipelines

---

## 19. What Sunkern Ships vs What the AI Builds

### Sunkern Ships (Built-in, Every Project Gets This)

**Framework:**
- App lifecycle, IoC container, facade
- HTTP server, router, middleware
- SQLite (CGo), migration engine, ORM/SQL builder
- In-memory cache (TTL + LRU)
- Reactive event bus + SSE support
- Cron scheduler (SQLite-backed)
- Job queue (SQLite-backed)
- Logging (dual: JSON stdout + JSONL file)
- Config system (defaults → yaml → env vars)
- Auth primitives (hashing, JWT, tokens)
- Base model (ID, timestamps, soft deletes)
- Email client (Resend)
- File upload/storage abstraction

**Service (built-in modules):**
- Admin module: auth (Super Admin + Admin roles), user management, log viewer, notification management, upload management, settings management, system metrics, possibly error tracker
- User-facing auth module (User role, soft-delete/block)
- User-facing notification module

**Admin UI:**
- Full admin dashboard with all management sub-modules

**Tooling:**
- Makefile, Dockerfile, CLAUDE.md, Sunkern Claude Code skill

### The AI Agent Builds (Per-Project)

- Domain-specific migrations (on top of built-in ones)
- Domain-specific models
- Domain-specific service modules (APIs, cron, workers, business logic)
- Extended admin sub-modules for domain management
- Entire user-facing application (UI ships empty)
- Deployment configuration (when platform skills are added)

---

## 20. Open Questions (To Research & Clarify Later)

### IoC Container & Modules
- Container API design: typed resolution (Go generics?) vs string keys vs interface-based?
- Module lifecycle hooks: OnStart/OnStop like fx, or simpler?
- How opinionated should registration order and dependency resolution be?
- Should modules declare dependencies explicitly, or resolve at runtime?

### Database & ORM
- Migration format: Go files with up/down functions, or raw SQL files, or both?
- ORM scope: models as structs with tags? Code generation? How much magic?

### HTTP & API
- API style deep research: response envelope, pagination, error format — optimized for React Query consumption
- Standard error type design for Go

### Event System
- Exact semantics: fan-out? topic-based? typed events?
- SSE: framework-level support or module-level?

### Queue & Cron
- Priority levels? Delayed jobs? Unique job constraints?

### Auth
- Access + refresh token strategy details. Token rotation? Revocation list?
- API tokens: scoped permissions or full access?
- Firebase integration: should the framework abstract social login, or is it purely a service concern?

### Error Tracking
- Feasibility of built-in Sentry-like error tracker in admin dashboard
