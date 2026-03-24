# Session Notes

> This file is the communication channel between autonomous sessions.
> Each session reads this at startup and updates it before ending.
> Keep it concise — this must stay useful across dozens of sessions.

---

## Current State

**Last session**: 2026-03-24 — Session 1 (framework/http maturation)
**Working tree**: clean
**Branch**: with-experiment

## Package Maturity Tracker

| Package | Maturity | Last Touched | Notes |
|---------|----------|--------------|-------|
| `framework/app` | Initial | — | Module lifecycle works, needs polish |
| `framework/container` | Initial | — | Generic DI works, needs lifecycle review |
| `framework/config` | Initial | — | 3-layer resolution works |
| `framework/log` | Initial | — | Dual output works |
| `framework/http` | **Growing** | 2026-03-24 | Route groups, request binding, response helpers, error types, server timeouts |
| `framework/http/middleware` | Initial | — | RequestID + RequestLogger only |
| `framework/db` | Initial | — | Query builder works for basics |
| `framework/sqlite` | Initial | — | Dual pool works |
| `framework/sqlite/driver` | Initial | — | CGo binding works |
| `framework/sqlite/migrate` | Initial | — | SQL migration engine works |

## Friction Log

<!-- Friction discovered during playground app testing. Format:
- [package] description of friction (session date)
-->

- [db] No `NewBaseModel()` constructor — creating a BaseModel requires manually setting ID, CreatedAt, UpdatedAt. Should add a convenience constructor. (2026-03-24)
- [db] `db.QueryOne[int]` with scalar types panics — scanner expects struct with db tags, not plain types. Only `db.Count()` works for counting. The type constraint on QueryOne should be documented or restricted. (2026-03-24)
- [http] Skill docs (sunkern-go-best-practices) still show old `server.Mux().HandleFunc(...)` pattern instead of new `server.Group(...)` pattern — permission denied when trying to update. Need to update skill rules. (2026-03-24)

## Performance Baselines

<!-- Load test results. Format:
- [package/endpoint] req/s, p99 latency, memory RSS (session date)
-->

- [http/GET list] 23,590 req/s, p99 5.1ms — paginated list with COUNT + SELECT (2026-03-24)
- [http/POST create] 30,578 req/s, p99 2.8ms — JSON bind + INSERT (2026-03-24)
- [http/GET single] 60,132 req/s, p99 2.5ms — single item by path param (2026-03-24)
- [memory] 49 MB RSS after 300k+ writes under sustained load (2026-03-24)

## Design Decisions

<!-- Key decisions and rationale so future sessions don't reverse them. Format:
- [package] decision — why (session date)
-->

- [http] RouteGroup path "/" maps to exact prefix (no trailing slash) — Go 1.22 ServeMux treats "/" as subtree match, so special-casing prevents accidental catch-all registration. "/{$}" available for explicit trailing-slash match. (2026-03-24)
- [http] Response envelope: `{"data": ...}` for success, `{"error": {"code": "...", "message": "..."}}` for errors — consistent, React Query friendly. JSONList adds `"pagination"` sibling. (2026-03-24)
- [http] APIError is the standard error type. Sentinels (ErrNotFound etc.) + WithMessage() for custom messages. Any non-APIError passed to Error() becomes 500. (2026-03-24)
- [http] Server timeouts: Read 15s, Write 15s, Idle 60s — production defaults per Go best practices. (2026-03-24)

## Next Priorities

<!-- What the last session thinks should come next, in order -->

1. Add `db.NewBaseModel()` convenience constructor (friction from this session)
2. Revisit `framework/db` — the query builder is the next most impactful package, needs maturation (Limit/Offset ergonomics, transactions, better error messages)
3. `framework/http/middleware` — add more middleware (CORS, rate limiting, recovery from panics)
4. `framework/container` / `framework/app` — review lifecycle hooks ordering, test isolation patterns
5. Update sunkern-go-best-practices skill with new HTTP patterns (blocked on permissions this session)

## In-Progress Work

<!-- If a session timed out, describe what's in the dirty working tree -->

(none)
