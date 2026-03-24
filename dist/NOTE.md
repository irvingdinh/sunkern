# Session Notes

> This file is the communication channel between autonomous sessions.
> Each session reads this at startup and updates it before ending.
> Keep it concise — this must stay useful across dozens of sessions.

---

## Current State

**Last session**: 2026-03-24 — Session 3 (framework/http/middleware maturation)
**Working tree**: clean
**Branch**: with-experiment

## Package Maturity Tracker

| Package | Maturity | Last Touched | Notes |
|---------|----------|--------------|-------|
| `framework/app` | Initial | — | Module lifecycle works, needs polish |
| `framework/container` | Initial | — | Generic DI works, needs lifecycle review |
| `framework/config` | Initial | — | 3-layer resolution works |
| `framework/log` | Initial | — | Dual output works |
| `framework/http` | **Growing** | Session 3 | Route groups, binding, response helpers, error types, timeouts, auto-OPTIONS registration |
| `framework/http/middleware` | **Growing** | Session 3 | RequestID, RequestLogger, Recover (panic recovery), CORS (functional options), RateLimit (token bucket) |
| `framework/db` | **Growing** | Session 2 | Query builder + NewBaseModel, auto-fill, QueryVal, soft delete, nullable columns, bool scan, auto-updated_at |
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
- [skill] Skill docs (sunkern-go-best-practices) show old `server.Mux().HandleFunc(...)` pattern — permission denied when trying to update. Blocked in Sessions 1-3. Need Irving to grant .claude/skills/ write permission. (2026-03-24)
- [db] NullStringColumn/NullIntColumn added but not yet stress-tested with real nullable data in playground (read path works, write path for NULL values untested). (Session 2)
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

## Next Priorities

<!-- What the last session thinks should come next, in order -->

1. **Revisit `framework/db`** with fresh eyes — test nullable column write paths, batch Model insert, cursor-based pagination patterns, subquery support
2. **`framework/container` / `framework/app`** — review lifecycle hooks ordering, test isolation patterns, error handling during boot
3. **Revisit `framework/http`** — request validation (beyond binding), file upload handling, SSE support
4. **`framework/db` advanced** — raw subqueries in WHERE, CASE expressions, window functions
5. **Revisit `framework/http/middleware`** — auth middleware (JWT validation, role-based), request timeout middleware
6. Update sunkern-go-best-practices skill (BLOCKED: need .claude/skills/ write permission)

## In-Progress Work

<!-- If a session timed out, describe what's in the dirty working tree -->

(none)
