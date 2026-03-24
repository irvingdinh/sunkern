# Session Notes

> This file is the communication channel between autonomous sessions.
> Each session reads this at startup and updates it before ending.
> Keep it concise — this must stay useful across dozens of sessions.

---

## Current State

**Last session**: 2026-03-24 — Session 10 (framework/http/middleware auth + JWT + MaxBytes)
**Working tree**: clean
**Branch**: with-experiment

## Package Maturity Tracker

| Package | Maturity | Last Touched | Notes |
|---------|----------|--------------|-------|
| `framework/app` | **Growing** | Session 4 | ModuleGroup boot rollback fix, lifecycle logging, module name tracking |
| `framework/container` | **Growing** | Session 4 | Override/OverrideSupply, named hooks, all framework hooks named |
| `framework/config` | **Growing** | Session 7 | Added Has, All, Keys, Sub, DataDir, EnvName, SetDefaults; Load creates data dir; improved panic messages |
| `framework/log` | **Growing** | Session 8 | Separate console/file levels (ConsoleLevel, FileLevel types), log file management (ListFiles, CleanOldFiles, OpenFile), JSONL entry parsing + querying (Entry, Query with level/search/user_id/request_id filter + pagination) |
| `framework/http` | **Maturing** | Session 10 | Revisited: Bind/BindForm now detect http.MaxBytesError and return 413 instead of 400. Previous: file upload handling (FormFile, FormFiles, BindForm, SaveFile, ValidateFile, DetectFileType), all sentinels |
| `framework/http/middleware` | **Maturing** | Session 10 | Revisited: JWT (HMAC-SHA256 sign/verify, Claims, context helpers), Auth (Bearer token extraction + verification + user_id logging), RequireRole (role-based 403), MaxBytes (body size limiter). Plus existing: RequestID, RequestLogger, Recover, CORS, RateLimit |
| `framework/db` | **Maturing** | Session 5 | Revisited: deterministic column order, generalized pointer handling, SetNull, ModelSlice batch insert, NullBoolColumn/NullFloatColumn, cursor pagination (After + HasMore) |
| `framework/sqlite` | Initial | Session 7 | Dual pool works; removed redundant data dir creation (config.Load handles it now); uses config.DataDir() |
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
- [http] Go's multipart.Writer.CreateFormFile always sets Content-Type to application/octet-stream. Real clients (curl, browsers) set it from filename. Framework documents this — prefer DetectFileType for user-facing uploads. (Session 9)
- ~~[http] ParseMultipartForm reads the entire request body before ValidateFile can reject oversized files~~ — RESOLVED in Session 10 (MaxBytes middleware + Bind/BindForm detect MaxBytesError → 413)

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

Session 6 (http revisit, contact manager with validation, 5k rows):
- [http/GET list] 7,283 req/s, p99 17.9ms — paginated, 5k rows, query validation active
- [http/POST reject] 73,944 req/s, p99 3.5ms — validation rejection path (4 errors per request)
- [http/GET single] 52,251 req/s, p99 2.7ms — single read by ID
- [memory] 42 MB RSS after 25k+ writes and 90k+ load test requests
- [validation overhead] negligible — 74k req/s rejection path vs 52k baseline GET shows validation adds <0.1ms

Session 7 (config, feature-flag app with config introspection, 100 DB rows):
- [config/GET all] 65,630 req/s, p99 2.6ms — All() with 10 keys, env override check
- [config/GET keys] 67,625 req/s, p99 2.5ms — Keys() sorted, 10 keys with EnvName mapping
- [config/GET sub] 67,625 req/s, p99 2.5ms — Sub("feature") returning 3 keys
- [config/GET features] 65,461 req/s, p99 2.5ms — Has() + Sub() + GetOr() combined
- [config/GET health] 66,270 req/s, p99 2.5ms — GetOr() for app name/version
- [settings/GET list] 10,148 req/s, p99 — list 100 rows, sorted by key
- [settings/GET single] 56,001 req/s, p99 2.7ms — single read by key
- [settings/PUT upsert] 36,810 req/s, p99 — read-then-write upsert
- [memory] 30 MB RSS after 160k+ requests
- [race] No data races detected with -race flag on concurrent config access

Session 8 (log, event generator with log viewer, 11k log entries):
- [log/POST create] 27,595 req/s, p99 8.3ms — event INSERT + slog.InfoContext with request_id
- [log/POST burst] 30,241 req/s, p99 — 200 events with mixed log levels (DEBUG/INFO/WARN/ERROR) and user_ids
- [log/GET files] 57,640 req/s, p99 2.8ms — ListFiles() directory scan
- [log/GET levels] 66,475 req/s, p99 2.4ms — container.MustMake + LevelVar.Level()
- [log/GET query-all] 106 req/s, p99 239ms — Query 11k JSONL entries, return 20 (IO-bound full file scan)
- [log/GET query-error] 99 req/s, p99 259ms — Query 11k entries filtered to ERROR level
- [log/GET query-search] 85 req/s, p99 300ms — Query 11k entries with text search
- [log/GET query-user] 82 req/s, p99 305ms — Query 11k entries filtered by user_id
- [log/single query] 41ms single request latency for 11k entries (3.9 MB JSONL file)
- [memory] 70 MB RSS after 6200+ writes + 11k log entries + concurrent query load
- [race] No data races detected with -race flag on all log package tests

Session 9 (http upload, file locker app with BindForm + SaveFile + ValidateFile, 5100 files):
- [upload/POST single] 10,201 req/s, P99 5.9ms — BindForm (title + file), ValidateFile, DetectFileType, SaveFile, DB INSERT
- [upload/POST reject-413] 433 req/s, P99 34.7ms — 11 MiB body parsed before 413 rejection (IO-bound by body transfer)
- [upload/GET list] 15,396 req/s, P99 8.6ms — paginated list on 5100 rows
- [upload/GET download] 49,902 req/s, P99 3.2ms — http.ServeFile for 33-byte PNG
- [upload/GET single] P99 2.9ms — single file metadata read
- [memory] 787 MB RSS peak after 2000 × 11 MiB rejection test (Go hadn't returned memory to OS yet; normal upload load used ~50 MB)
- [race] No data races detected with -race flag on concurrent upload + read

Session 10 (middleware auth, notes app with JWT auth + role-based access, 11k notes):
- [auth/POST login] 47,940 req/s, p99 2.9ms — JWT SignToken (HMAC-SHA256)
- [auth/GET list] 5,215 req/s, p99 24.7ms — JWT verify + paginated list on 11k rows
- [auth/POST create] 28,875 req/s, p99 7.5ms — JWT verify + JSON bind + INSERT
- [auth/401 reject] 65,934 req/s, p99 2.5ms — Auth middleware short-circuit (no token)
- [auth/403 reject] 57,258 req/s, p99 2.6ms — JWT verify + RequireRole short-circuit
- [auth/GET admin] 3,628 req/s, p99 41.6ms — JWT verify + role check + COUNT on 11k rows
- [memory] 56 MB RSS after 50k+ writes and 60k+ load test requests
- [race] No data races detected with -race flag on all framework tests
- [jwt overhead] ~0.5ms per verify (compare login 48k to 401 reject 66k — crypto is cheap)

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
- [http] Validation runs automatically inside Bind/BindQuery — zero-effort for the AI agent. Uses `validate` struct tags. Tag cache via sync.Map parsed once per type. (Session 6)
- [http] Zero-value fields skip non-required rules — matches Gin convention. For optional fields, zero = "not provided", so min/max/email/etc are skipped. Use `required` to enforce presence. (Session 6)
- [http] Pointer fields: nil = zero (required check); non-nil = dereference and validate. Perfect for PATCH update structs where nil means "don't update" and non-nil means "update to this value". (Session 6)
- [http] Field name resolution for error messages: json tag > query tag > lowercased Go name. Matches the binding context — JSON body uses json tags, query params use query tags. (Session 6)
- [http] Validation errors return 422 with `{"error": {"code": "validation_error", "message": "Validation failed", "details": [...]}}`. Field-level details include field name, rule name, and human-readable message. (Session 6)
- [http] Validator interface allows custom cross-field validation after tag-based validation passes. Used for bulk operations (validate each item in an array) or business rules that tags can't express. (Session 6)
- [http] Embedded structs are recursed — enables shared paginationParams with validation reused across list endpoints. (Session 6)
- [config] Load() creates the data directory — downstream packages (sqlite, log) no longer need to. Single point of responsibility. (Session 7)
- [config] All() and Keys() only enumerate keys from defaults + config file, then check env overrides. Keys that exist solely as env vars (never registered via SetDefault or config.json) are not discoverable — this is intentional; it avoids scanning the entire environment. (Session 7)
- [config] Sub() returns a flat map with prefix stripped — not a nested config instance. Simple and sufficient for passing config sections to subsystems. Returns nil (not empty map) when no keys match. (Session 7)
- [config] Panic messages in Get() now include the env var name as a hint — `config: key "jwt.secret" not found (set JWT_SECRET env var or add to config.json)`. Coercion failures include the raw value for debugging. (Session 7)
- [log] Separate console and file levels: `log.level` controls file (default INFO), `log.console.level` controls console (defaults to log.level when not set). Backward compatible — setting only LOG_LEVEL still works for both sinks. (Session 8)
- [log] ConsoleLevel and FileLevel are exported named types wrapping slog.LevelVar. Supplied to container as *ConsoleLevel and *FileLevel. Embedding promotes Set()/Level() methods — callers use cl.Set(slog.LevelWarn) directly. (Session 8)
- [log] ListFiles returns newest-first — date strings sort lexicographically. CleanOldFiles uses strict less-than on cutoff date, so retentionDays=7 keeps today + last 7 days (8 files total). (Session 8)
- [log] Entry parsing uses two-pass: unmarshal into map[string]any, extract known fields (time, level, msg, source, request_id, user_id), put rest in Extra. Custom MarshalJSON re-merges for flat JSON output matching original JSONL structure. (Session 8)
- [log] Query scans the entire file sequentially — IO-bound at ~100 req/s for 11k entries (4MB). Acceptable for admin viewer. Future optimization: mmap, line indexing, or in-memory cache if needed. (Session 8)
- [log] QueryOptions.Level is a string (not slog.Level) — empty string means "all levels". Avoids the zero-value problem where slog.LevelInfo=0 would accidentally filter out DEBUG. levelRank() maps strings to ordered ints for comparison. (Session 8)
- [http] File upload uses function-based API (FormFile, BindForm, SaveFile) consistent with existing Bind/BindQuery pattern — not method-based like Gin's Context. Keeps http package stateless. (Session 9)
- [http] BindForm uses `form` struct tag, parallel to `json` (Bind) and `query` (BindQuery). Supports *multipart.FileHeader and []*multipart.FileHeader for file fields alongside scalar text fields. (Session 9)
- [http] FormFile/FormFiles access r.MultipartForm.File directly instead of calling r.FormFile() — avoids unnecessary file opening. User opens when needed via FileHeader.Open(). (Session 9)
- [http] ValidateFile checks the Content-Type header (fast, no IO). DetectFileType reads first 512 bytes for real content sniffing. Two separate functions — user chooses security level. (Session 9)
- [http] SaveFile creates parent directories (0o755) — convenience for the common pattern of generating date-based upload paths. (Session 9)
- [http] DefaultMaxMemory = 32 MiB — consistent with Go stdlib default. Files under this stay in memory, larger ones spill to temp files on disk. (Session 9)
- [http] Field name resolution updated: json > query > form > lowercased Go name. Ensures validation errors use the correct binding-context name. (Session 9)

- [middleware] JWT lives in middleware package (not http or a new package) — middleware can't import parent http (circular dep), and JWT is primarily consumed by middleware + service handlers. SignToken/VerifyToken/ClaimsFromCtx all in one package. No new top-level packages per GUIDELINE. (Session 10)
- [middleware] JWT uses HMAC-SHA256 only — simplest algorithm, sufficient for standalone apps. Pre-computed base64url header constant avoids per-sign allocation. (Session 10)
- [middleware] Claims struct uses time.Time (ergonomic) but marshals as Unix timestamps (JWT standard). Extra map[string]any for custom claims — reserved keys (sub, role, exp, iat) cannot be overridden via Extra. (Session 10)
- [middleware] Auth middleware extracts Bearer token from Authorization header, verifies JWT, stores Claims in context (ClaimsFromCtx), and sets user_id for structured logging (sunkernlog.WithUserID). Two responsibilities in one middleware — justified because user_id in logs is always wanted when auth is present. (Session 10)
- [middleware] RequireRole uses map[string]struct{} for O(1) role lookup, pre-built at init time. Returns 401 if no claims (auth middleware not present), 403 if role doesn't match. (Session 10)
- [middleware] MaxBytes wraps r.Body with http.MaxBytesReader. The actual error surfaces when Bind/BindForm reads the body — they detect *http.MaxBytesError via errors.As and return ErrPayloadTooLarge (413) instead of ErrBadRequest (400). This addresses Session 9 friction about late rejection of oversized uploads. (Session 10)
- [middleware] Auth/RequireRole error responses use inline JSON (same pattern as recover.go, ratelimit.go) to avoid circular import with parent http package. Error codes match the sentinel names: "unauthorized", "forbidden". (Session 10)
- [middleware] Exported JWT error types (ErrTokenMalformed, ErrTokenExpired, ErrTokenInvalid, ErrSecretEmpty) allow service-layer code to distinguish failure modes — e.g., showing "Token expired" vs generic "Invalid token". (Session 10)

## Next Priorities

<!-- What the last session thinks should come next, in order -->

1. **Revisit `framework/http`** — SSE support for real-time events (event bus → SSE endpoint pattern from IDEA.md Section 9)
2. **Revisit `framework/http/middleware`** — request timeout middleware (context deadline for slow handlers), password hashing (bcrypt via stdlib crypto), API token middleware (separate from JWT — long-lived, revocable)
3. **`framework/db` advanced** — RETURNING clause (eliminate update-then-fetch pattern), subqueries in WHERE, CASE expressions
4. **Revisit `framework/app` / `framework/container`** — now Growing, revisit after other packages evolve
5. **Revisit `framework/config`** — consider config validation (type constraints, allowed values)
6. **Revisit `framework/log`** — consider Query performance optimization (mmap/indexing), log sampling for high-traffic paths
7. **Revisit `framework/db`** — fresh perspective on API ergonomics after http fully matures
8. Update sunkern-go-best-practices skill (BLOCKED: need .claude/skills/ write permission)

## In-Progress Work

<!-- If a session timed out, describe what's in the dirty working tree -->

(none)
