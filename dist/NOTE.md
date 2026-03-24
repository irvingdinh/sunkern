# Session Notes

> This file is the communication channel between autonomous sessions.
> Each session reads this at startup and updates it before ending.
> Keep it concise — this must stay useful across dozens of sessions.

---

## Current State

**Last session**: 2026-03-24 — Session 5 (framework/db revisit — nullable write paths, batch insert, cursor pagination)
**Working tree**: clean
**Branch**: with-experiment

## Package Maturity Tracker

| Package | Maturity | Last Touched | Notes |
|---------|----------|--------------|-------|
| `framework/app` | **Growing** | Session 4 | ModuleGroup boot rollback fix, lifecycle logging, module name tracking |
| `framework/container` | **Growing** | Session 4 | Override/OverrideSupply, named hooks, all framework hooks named |
| `framework/config` | Initial | — | 3-layer resolution works |
| `framework/log` | Initial | — | Dual output works |
| `framework/http` | **Growing** | Session 3 | Route groups, binding, response helpers, error types, timeouts, auto-OPTIONS registration |
| `framework/http/middleware` | **Growing** | Session 3 | RequestID, RequestLogger, Recover (panic recovery), CORS (functional options), RateLimit (token bucket) |
| `framework/db` | **Maturing** | Session 5 | Revisited: deterministic column order, generalized pointer handling, SetNull, ModelSlice batch insert, NullBoolColumn/NullFloatColumn, cursor pagination (After + HasMore) |
| `framework/sqlite` | Initial | — | Dual pool works |
| `framework/sqlite/driver` | Initial | — | CGo binding works |
| `framework/sqlite/migrate` | Initial | — | SQL migration engine works |

## Friction Log

<!-- Friction discovered during playground app testing. Format:
- [package] description of friction (session date)
-->

- ~~[db] No `NewBaseModel()` constructor~~ — RESOLVED in Session 2
- ~~[db] `db.QueryOne[int]` with scalar types panics~~ — RESOLVED in Session 2 (added QueryVal[T])
- ~~[http] Go ServeMux doesn't route OPTIONS to method-specific handlers, breaking CORS preflight~~ — RESOLVED in Session 3 (RouteGroup auto-registers OPTIONS handlers)
- ~~[app] ModuleGroup doesn't roll back booted children when a sibling fails Boot~~ — RESOLVED in Session 4 (tracks booted children, rolls back + clears to prevent double-shutdown)
- ~~[db] NullStringColumn/NullIntColumn added but not yet stress-tested with real nullable data in playground~~ — RESOLVED in Session 5 (tested all nullable types: *string, *int64, *float64, *bool write/read paths verified with 125k rows)
- [skill] Skill docs (sunkern-go-best-practices) show old `server.Mux().HandleFunc(...)` pattern — permission denied when trying to update. Blocked in Sessions 1-5. Need Irving to grant .claude/skills/ write permission. (2026-03-24)
- [db] Insert().Model() auto-fill only works for BaseModel fields (id, created_at, updated_at). Custom fields with zero-value defaults still need manual handling. Acceptable trade-off. (Session 2)

## Performance Baselines

<!-- Load test results. Format:
- [package/endpoint] req/s, p99 latency, memory RSS (session date)
-->

Session 1 (http, simple users module):
- [http/GET list] 23,590 req/s, p99 5.1ms — paginated list with COUNT + SELECT
- [http/POST create] 30,578 req/s, p99 2.8ms — JSON bind + INSERT
- [http/GET single] 60,132 req/s, p99 2.5ms — single item by path param
- [memory] 49 MB RSS after 300k+ writes under sustained load

Session 2 (db, bookmark manager with 10k+ rows):
- [db/GET list] 10,445 req/s, p99 12.9ms — COUNT + paginated SELECT on 10k+ rows
- [db/POST create] 25,641 req/s, p99 3.5ms — JSON bind + INSERT with auto-fill
- [db/GET single] 56,153 req/s, p99 2.8ms — single read by ID
- [db/GET stats] 10,807 req/s, p99 11.0ms — aggregate COUNT + SUM on 10k+ rows
- [memory] 45 MB RSS after 15k+ writes under sustained load

Session 3 (middleware, link shortener with CORS + RateLimit + Recover):
- [middleware/GET list] 21,101 req/s, p99 5.8ms — with CORS + RateLimit + Recover active
- [middleware/POST create] 42,201 req/s, p99 1.4ms — with all middleware
- [middleware/GET single] 53,997 req/s, p99 2.9ms — with all middleware
- [middleware/panic] 28,696 req/s, p99 2.4ms — every request panics, Recover catches all
- [middleware/ratelimit] 15/15 burst allowed, 185/200 correctly throttled at 10 req/s burst 15
- [memory] 28 MB RSS under sustained load

Session 4 (app+container, task tracker with ModuleGroup + heartbeat hook):
- [app/GET list] 8,185 req/s, p99 6.1ms — paginated list on 10k+ rows
- [app/POST create] 29,210 req/s, p99 7.5ms — JSON bind + INSERT
- [app/GET health] 69,828 req/s, p99 2.5ms — health endpoint with atomic counter
- [app/GET admin-stats] 18,994 req/s, p99 5.3ms — ModuleGroup sub-module with raw SQL
- [memory] 46 MB RSS after 30k+ writes under sustained load
- [lifecycle] Graceful shutdown drained background heartbeat goroutine cleanly (113 beats)

Session 5 (db revisit, notes app with all nullable types, 125k rows):
- [db/GET list] 22,153 req/s, p99 5.8ms — cursor-paginated, 20 items on 125k rows
- [db/GET filtered] ~filtered by category, p99 16.1ms — WHERE on nullable column, 125k rows
- [db/POST create] 26,304 req/s, p99 8.4ms — INSERT with 6 nullable fields
- [db/GET single] 55,528 req/s, p99 2.7ms — single read by ID
- [db/GET stats] 506 req/s, p99 83ms — AVG+MAX+SUM aggregates on 125k rows (expected: full table scan)
- [db/POST batch] ~batch 50 notes per req, p99 16.4ms — ModelSlice with mixed nullable fields
- [memory] 85 MB RSS after 125k+ writes under sustained load

## Design Decisions

<!-- Key decisions and rationale so future sessions don't reverse them. Format:
- [package] decision — why (session date)
-->

- [http] RouteGroup path "/" maps to exact prefix (no trailing slash) — Go 1.22 ServeMux treats "/" as subtree match, so special-casing prevents accidental catch-all registration. "/{$}" available for explicit trailing-slash match. (Session 1)
- [http] Response envelope: `{"data": ...}` for success, `{"error": {"code": "...", "message": "..."}}` for errors — consistent, React Query friendly. JSONList adds `"pagination"` sibling. (Session 1)
- [http] APIError is the standard error type. Sentinels (ErrNotFound etc.) + WithMessage() for custom messages. Any non-APIError passed to Error() becomes 500. (Session 1)
- [http] Server timeouts: Read 15s, Write 15s, Idle 60s — production defaults per Go best practices. (Session 1)
- [http] RouteGroup auto-registers OPTIONS handlers for every method-specific route — Go's ServeMux doesn't route OPTIONS to "GET /path" handlers, so CORS preflight would bypass group middleware. Auto-OPTIONS fixes this transparently. Shared optionsRegistry prevents duplicate registration panics across sub-groups. (Session 3)
- [middleware] Recover is a global middleware (inside RequestLogger, outside mux). Execution order: RequestID → RequestLogger → Recover → mux. This ensures panics are caught AND the 500 response is logged by RequestLogger. (Session 3)
- [middleware] CORS uses functional options (WithAllowOrigins, WithAllowCredentials, etc.). Pre-joins header values at init time to avoid per-request allocations. Origin lookup uses map for O(1) checks. (Session 3)
- [middleware] RateLimit uses token bucket per-IP with lazy cleanup (no background goroutine). Default key extraction: X-Real-IP → X-Forwarded-For → RemoteAddr, suited for reverse proxy deployments (Fly.io, Railway). (Session 3)
- [middleware] Error responses in middleware are inline JSON (not using http.Error()) because middleware package can't import parent http package (circular dependency). (Session 3)
- [db] Insert().Model() auto-fills BaseModel fields (ID, timestamps) when pointer is passed — the caller's struct stays in sync with what gets inserted. Non-pointer path also auto-generates empty IDs but can't write back. (Session 2)
- [db] Update().Build() auto-appends `updated_at = NOW()` if not explicitly set — ensures BaseModel timestamps are always refreshed on writes. (Session 2)
- [db] SoftDelete/Restore are UpdateBuilder wrappers, not new builder types — keeps the API surface small and composable. (Session 2)
- [db] Scanner handles int64→bool for SQLite INTEGER booleans — required for BoolColumn fields since SQLite has no native boolean type. (Session 2)
- [container] Override replaces registrations silently (no panic), intended for testing and env switching. Provide panics on duplicate — this is the correct default for production code where double-registration is a bug. (Session 4)
- [container] Named hooks are optional (Name field). When set, error messages use the name; when empty, they fall back to positional index. All framework hooks (http, sqlite, log) are named. (Session 4)
- [app] ModuleGroup tracks booted children separately from registered children. On partial boot failure: roll back booted children, clear the list. On Shutdown: only shut down booted children. Prevents double-shutdown when parent also calls Shutdown. (Session 4)
- [db] Model()/SetModel() emit columns in sorted order — map iteration is non-deterministic in Go, sorted keys make generated SQL stable and debuggable. (Session 5)
- [db] Nil pointer fields skip in Model() (single-row INSERT, let DEFAULT apply), insert NULL in ModelSlice() (multi-row INSERT, all rows need same columns). Non-nil pointers always dereference before binding. (Session 5)
- [db] SetNull(col) is syntactic sugar for SetExpr(col, Raw("NULL")) — provides explicit intent for clearing nullable fields, since SetModel() skips nil pointers (meaning "don't update"). (Session 5)
- [db] After(cursor, limit) fetches limit+1 rows; HasMore() trims and returns hasMore flag. Caller manages ORDER BY for flexibility (forward vs backward pagination). (Session 5)

## Next Priorities

<!-- What the last session thinks should come next, in order -->

1. **Revisit `framework/http`** — request validation (beyond binding), file upload handling, SSE support
2. **`framework/config` / `framework/log`** — still at Initial maturity, need playground stress-testing and API review
3. **`framework/db` advanced** — RETURNING clause (eliminate update-then-fetch pattern), subqueries in WHERE, CASE expressions
4. **Revisit `framework/http/middleware`** — auth middleware (JWT validation, role-based), request timeout middleware
5. **Revisit `framework/app` / `framework/container`** — now Growing, revisit after other packages evolve
6. **Revisit `framework/db`** — revisit after http/config/log mature, fresh perspective on API ergonomics
7. Update sunkern-go-best-practices skill (BLOCKED: need .claude/skills/ write permission)

## In-Progress Work

<!-- If a session timed out, describe what's in the dirty working tree -->

(none)
