# Session Notes

> This file is the communication channel between autonomous sessions.
> Each session reads this at startup and updates it before ending.
> Keep it concise — this must stay useful across dozens of sessions.

---

## Current State

**Last session**: 2026-03-25 — Session 43 (framework/http revisited — SSE Broker, ResponseRecorder)
**Working tree**: clean
**Branch**: with-experiment

## Package Maturity Tracker

| Package | Maturity | Last Touched | Notes |
|---------|----------|--------------|-------|
| `framework/app` | **Maturing** | Session 38 | Revisited: Tagger optional interface ([]string tags for admin categorization), DependencyDeclarer optional interface (module-level dependency validation between register and boot), When(bool, Module) conditional wrapper (disabled modules skip lifecycle, preserve metadata for introspection), ModuleStatus/ModuleInfo types with JSON tags for admin API, App.ModuleInfo() returns all modules with status/tags/deps, duplicate module name validation in register phase, disabled modules excluded from dependency validation both directions, moduleNames() shows "(disabled)" suffix, 18 new tests (39 total). Load tested: ModuleInfo 67K req/s p99 2.7ms. Previous (Session 30): PostBooter, PreShutdowner, phase timing, ModuleGroup delegates. Previous (Session 25): health check system, ReadyCh, Env/Version. Previous (Session 13): *App supplied to container, boot/shutdown timing. Previous: ModuleGroup boot rollback fix |
| `framework/container` | **Maturing** | Session 37 | Revisited: circular dependency detection (prevents deadlocks — panics with clear "A → B → A" chain message), automatic dependency tracking during provider resolution (recorded as sorted unique []string per service), DependencyGraph() adjacency-list method for admin dashboards, Deps field on ServiceInfo with JSON serialization, Container.Reset() instance method for non-global container test isolation, goID()-based per-goroutine resolution tracking (zero overhead on cached hits — fast path skips goID entirely), 11 new tests (48 total). Previous (Session 26): ServiceInfo enriched with Kind/Caller/ErrorText; HookReport with per-hook timing; caller tracking in Provide/Supply/Override. Previous (Session 13): introspection APIs (Keys, Inspect, Len), ServiceInfo/ServiceStatus types. Previous: Override/OverrideSupply, named hooks |
| `framework/config` | **Maturing** | Session 39 | Revisited: Describe(key, desc) attaches human-readable descriptions to config keys, Description(key) read accessor, Entry enriched with Description/DefaultValue/Overridden fields for admin dashboards, Diff(before, after []Entry) compares Export snapshots (returns added/removed/changed sorted by key), fixed Validate() bug that skipped Freeze() when no rules registered, fixed app.go SetDefault ordering (was before Load() which wipes state), all framework packages describe their config keys (23 keys total: 8 db, 4 http, 4 log, 2 app, 5 app-module), 15 new tests (52 total). Load tested: Export 45K req/s p99 3.4ms, Keys 65K req/s p99 2.6ms, Diff 42K req/s p99 3.2ms, Describe 67K req/s p99 2.5ms, Sub 65K req/s p99 2.5ms, 51 MB RSS under sustained load, zero data races. Previous (Session 29): Source tracking, MarkSensitive, Export, Freeze/IsFrozen. Previous (Session 16): config validation. Previous (Session 7): Has, All, Keys, Sub, DataDir, EnvName, SetDefaults |
| `framework/log` | **Maturing** | Session 40 | Revisited: per-request consistent sampling (FNV-1a hash of request_id for deterministic all-or-nothing trace sampling, falls back to counter when no request context), OnRotate(fn) callback fired asynchronously on daily file rotation (snapshot-copied under lock, goroutine dispatch), Query early termination for ascending Before-bounded scans (skip JSON parsing once past time window — 16.7x faster on 14MB/46K-entry file). 47 tests pass. Load tested: list 86K req/s p99 1.7ms, create 90K req/s p99 1.8ms, sampling-test 4.6K req/s p99 7.4ms, query-before 459 req/s p99 234ms (vs full-scan 27.5 req/s), 23 MB RSS. Previous (Session 31): samplingHandler atomic counters, mergedHandler.Enabled optimization, Query context cancellation, QueryResult, date validation. Previous (Session 17): buffered file writer. Previous (Session 8): dual levels, file management, JSONL parsing + querying |
| `framework/http` | **Maturing** | Session 43 | Revisited: SSE Broker — topic-based fan-out hub managing multiple SSE connections (NewBroker, Run, Subscribe, Publish, Broadcast, Stats). Single event-loop goroutine (channel-based, no locks for client/topic maps), per-client buffered channels with non-blocking drops for slow clients, atomic stats counters (clients, topics, published, errors). Options: WithHeartbeatInterval, WithClientBuffer. ErrBrokerStopped sentinel. ResponseRecorder — exported response inspection wrapper (NewResponseRecorder, Status, BytesWritten, Flush, Unwrap). 15 new tests (27 total in http package). Load tested (20 SSE clients): stats 71K req/s p99 1.1ms, POST+publish 21K req/s p99 3.5ms, broadcast 11K req/s p99 3.5ms. Scaled (200 SSE clients): POST+publish 17.6K req/s p99 4.0ms, broadcast 1.1K req/s p99 20ms, 60 MB RSS. Zero data races. Previous (Session 35): Chain, SkipIf, SkipPaths. Previous (Session 28): PaginationParams, embedded BindQuery, Created(), configurable timeouts. Previous (Session 14): SSE support. Previous (Session 10): Bind/BindForm MaxBytesError → 413 |
| `framework/http/middleware` | **Maturing** | Session 35 | Revisited: SubjectFromCtx(ctx) string extracts Claims.Subject directly (returns "" if no claims), RoleFromCtx(ctx) string extracts Claims.Role directly — eliminates ClaimsFromCtx+nil-check boilerplate. 5 new tests (28 total). Previous (Session 28): writeErrorJSON shared helper. Previous (Session 15): Timeout, PBKDF2-SHA256, APIToken, GenerateToken. Previous (Session 10): JWT, Auth, RequireRole, MaxBytes. Plus existing: RequestID, RequestLogger, Recover, CORS, RateLimit |
| `framework/db` | **Maturing** | Session 36 | Revisited: RunTxVal[T] generic transaction with return value (same rollback/panic semantics as RunTx), Exists() optimized with SELECT EXISTS(SELECT 1 ... LIMIT 1) instead of COUNT(*) (stops at first match), FindByID[T] convenience with optional scopes (replaces 4-line Select+Where+Limit+QueryOne), DeleteByID hard-delete (parallels SoftDeleteByID), service/users controller updated to use FindByID. Previous (Session 27): INSERT...SELECT, DoUpdateAll, ConflictBuilder.Where, ConflictWhere, Excluded(), Returning(...Expr), ReturningStar(). Previous (Session 24): CTEs, With() on all builders. Previous (Session 23): set operations, FILTER, Query interface. Previous (Session 22): window functions, GROUP_CONCAT. Previous (Session 21): JOIN ergonomics. Previous (Session 12): RETURNING, subqueries, CASE. Previous (Session 5): nullable types, cursor pagination |
| `framework/sqlite` | **Maturing** | Session 42 | Revisited: DB.SetUpdateHook(fn) exposes driver update hook at sqlite package level with automatic write-pool connection recycling (SetMaxIdleConns 0→1 forces pool to create fresh connection via connector), WAL hook integration with maintenance goroutine for proactive checkpoint triggering (non-blocking signal channel, coalesced via buffered chan struct{} cap 1), maintenance.checkpoint() extracted as shared method (called from both periodic tick and WAL signal), walPageLimit derived from byte threshold / page_size for frame-count comparison in hook. Load tested: POST with update hook 28K req/s p99 7.6ms (~7% overhead vs baseline), proactive checkpoint triggered at 100.6 MB WAL, 39.8 MB RSS after 35K writes. Previous (Session 34): db.trace config key. Previous (Session 32): Connector per-connection PRAGMAs, sql.OpenDB, GetPragma/SetPragma/Pragmas, TableStats, Stats.CurrentPragmas. Previous (Session 18): maintenance goroutine, Health(ctx). Previous (Session 11): Stats, Checkpoint, Optimize, IntegrityCheck, Backup. |
| `framework/sqlite/driver` | **Maturing** | Session 42 | Revisited: sqlite3_update_hook — row-level INSERT/UPDATE/DELETE change notifications. UpdateAction type (ActionInsert=18, ActionDelete=9, ActionUpdate=23) with String() method, UpdateInfo struct (Action, DBName, Table, RowID), UpdateFunc callback type, Connector.SetUpdateHook(fn), _update_trampoline in hook.c, sunkernUpdateCallback //export. Follows exact same CGo pattern as trace/busy/WAL hooks (C trampoline → cgo.Handle → connHooks → Go callback). connHooks extended with update field, installHooks and Connect() snapshot updated. Load tested: ~7% write throughput overhead (28K vs 30K req/s baseline), change feed ring buffer at 30K req/s read. Previous (Session 34): trace_v2, busy_handler, wal_hook, CGo callback trampolines. Previous (Session 32): Connector per-connection PRAGMAs, DBSTAT_VTAB. Previous (Session 20): structured Error type, context cancellation, MemoryUsed/MemoryHighwater. |
| `framework/sqlite/migrate` | **Maturing** | Session 41 | Revisited: sync.Mutex locking on all mutation methods (Up/UpTo/Down/DownTo/Redo/DryRun — prevents concurrent migration races), BeforeEachFunc/AfterEachFunc hooks (OnBeforeEach can abort, OnAfterEach informational — fired for all operations including Redo's down+up phases), Direction.String() method, DryRun() validates pending migrations in rolled-back transaction (PlannedMigration with Valid/Error, skips dependents on first failure). Load tested: Status 37K req/s p99 1.3ms, Version 44K req/s p99 1.0ms, Validate 39K req/s p99 1.2ms, hooks (in-memory) 70K req/s p99 0.9ms, 25 MB RSS. Concurrent test: 10 goroutines all succeed (0 errors). Previous (Session 33): Validate, HasPending, Status orphaned records, appliedRecords refactor. Previous (Session 19): SHA-256 checksums, UpTo/DownTo/Version/Redo. Previous (Session 11): migration timing, Pending(), error context. |

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

Session 11 (sqlite ops, bookmark manager with 25k rows):
- [sqlite/GET stats] 55,751 req/s, p99 3.0ms — PRAGMA queries + os.Stat + pool stats
- [sqlite/GET integrity] 878 req/s, p99 5.5ms — PRAGMA quick_check (full DB scan, admin-only)
- [sqlite/POST checkpoint] sub-ms — WAL checkpoint with TRUNCATE mode
- [sqlite/POST optimize] sub-ms — PRAGMA optimize
- [sqlite/POST backup] p99 20ms — VACUUM INTO for 25k rows
- [sqlite/GET list] 11,182 req/s, p99 7.8ms — paginated on 25k rows
- [sqlite/POST create] 26,533 req/s, p99 8.3ms — JSON bind + INSERT
- [memory] 65 MB RSS after 25k+ writes and 60k+ load test requests

Session 12 (db revisit, task tracker with tags, 15k+ rows):
- [db/POST insert-returning] 21,201 req/s, p99 10ms — INSERT + RETURNING scan, 15k rows
- [db/PATCH update-returning] ~50k req/s, p99 7.7ms — single-row UPDATE RETURNING
- [db/GET exists-subquery] 44,759 req/s, p99 4.1ms — EXISTS correlated subquery (small result)
- [db/GET in-subquery] 1,524 req/s, p99 114ms — IN subquery returning ~1200 rows (IO-bound, JSON serialization)
- [db/GET case-summary] 1,546 req/s, p99 179ms — CASE + GROUP BY full table scan on 15k rows (expected)
- [memory] 94 MB RSS after 15k+ writes and 30k+ load test requests
- [race] No data races detected with -race flag on concurrent RETURNING + subquery + CASE

Session 13 (app+container revisit, system monitor with ModuleGroup + heartbeat worker, 5k+ rows):
- [container/GET health] 60,161 req/s, p99 2.4ms — Keys() + Len() + App.Ready()
- [container/GET services] 59,154 req/s, p99 2.6ms — Inspect() with 5 services
- [container/GET hooks] ~64k req/s, p99 2.6ms — Hooks() returning 3 hooks
- [heartbeat/GET status] 64,418 req/s, p99 2.6ms — atomic counter read
- [notes/GET list] 16,785 req/s, p99 7.5ms — paginated list on 5k rows
- [notes/POST create] ~32k req/s, p99 2.8ms — JSON bind + INSERT
- [memory] 15 MB RSS after 100k+ requests, 34 MB with 5k+ rows under sustained load
- [race] No data races detected with -race flag on concurrent introspection + writes + heartbeat
- [shutdown] Heartbeat worker detected ShuttingDown() signal and terminated cleanly

Session 14 (http SSE, stock ticker with price hub + 5 tickers):
- [sse/50 connections] 0 errors, connect p50 1.7ms, p99 2.1ms — 20 events/client over 10s
- [sse/200 connections] 0 errors, connect p50 9.9ms, p99 14ms — 20 events/client over 10s
- [sse/500 connections] 0 errors, connect p50 21ms, p99 50ms — 20 events/client over 10s
- [sse/events] 0 dropped events across all tests (19,100 total events sent, 850 total connections)
- [sse+crud/GET list] 18,327 req/s, p99 5.2ms — with 100 active SSE connections (SSE does not degrade CRUD)
- [memory] 42.6 MB RSS after 850+ SSE connections + sustained load
- [race] No data races detected with -race flag on concurrent SSE + CRUD

Session 15 (middleware auth, secure notes with password hashing + API tokens + timeout, 10k+ notes):
- [password/POST login] 123 req/s, p99 96ms — PBKDF2-SHA256 600k iterations (CPU-bound by design, ~80ms/hash)
- [jwt/GET list] 16,085 req/s, p99 7.4ms — JWT verify + Timeout(5s) + paginated list on 202 rows
- [apitoken/GET list] 13,498 req/s, p99 8.3ms — API token DB lookup + Timeout(5s) + paginated list on 202 rows
- [jwt/POST create] 27,606 req/s, p99 3.0ms — JWT verify + JSON bind + INSERT
- [apitoken/POST create] 22,722 req/s, p99 3.7ms — API token DB lookup + JSON bind + INSERT
- [401 reject] 61,065 req/s, p99 4.3ms — Auth middleware short-circuit (no token)
- [apitoken/401 reject] 50,745 req/s, p99 2.9ms — API token middleware with invalid token (DB miss)
- [memory] 31 MB RSS after 10k+ writes under sustained load
- [race] No data races detected with -race flag on concurrent JWT + API token + login + writes
- [apitoken overhead] ~16% vs JWT for reads (~19% for writes) — DB lookup per request, acceptable trade-off

Session 16 (config validation, feature flag app with validated config, 500 flags):
- [config/GET introspection] 59,598 req/s, p99 2.6ms — config.Sub + config.Has + config.Keys + config.GetOr
- [flags/GET list] 19,156 req/s, p99 6.7ms — paginated list on 500 rows
- [flags/GET single] 53,553 req/s, p99 2.9ms — single read by ID
- [memory] 27 MB RSS after 500+ writes and 40k+ load test requests
- [race] No data races detected with -race flag on concurrent config reads + CRUD
- [validation] Invalid config caught at boot: multi-error panic with key + env var name + violation per rule

Session 17 (log revisit, log analytics app with 131k entries, 36MB log file):
- [log/POST generate×200] 724 req/s, p99 65ms — 200 slog calls per request (~145k log writes/sec through buffered writer)
- [log/GET query-asc] 76 req/s, p99 150ms — Query with limit=20 on 15k entries (5.4MB), asc order
- [log/GET query-desc] 92 req/s, p99 135ms — Query with limit=20 on 15k entries, desc order
- [log/GET query-asc-131k] 8 req/s, p99 629ms — Query with limit=20 on 131k entries (36MB), asc order
- [log/GET query-desc-131k] 9 req/s, p99 620ms — Query with limit=20 on 131k entries, desc order
- [log/GET count-131k] 8 req/s, p99 659ms — CountOnly on 131k entries (same IO, skips entry allocation)
- [health] 67,808 req/s, p99 0.4ms — baseline
- [memory] 996 MB RSS peak after 131k write burst + desc query on full file (Go not returning to OS; normal workload ~50MB)
- [race] No data races detected with -race flag on all log package tests (35 tests)
- [write scaling] Buffered writer batches ~100-300 entries per syscall (64KB buffer). Session 8 baseline was unbuffered.
- [query scaling] Linear with file size: 15k entries → 76 req/s, 131k entries → 8 req/s (~12x, matches 12x data growth)

Session 18 (sqlite revisit, bookmark manager with health + DB ops, 5500 rows):
- [sqlite/GET health] 69,424 req/s, p99 2.6ms — dual pool PingContext (write + read)
- [sqlite/GET stats] 57,075 req/s, p99 1.2ms — PRAGMA queries + file stats + pool stats
- [sqlite/GET list] 20,613 req/s, p99 6.0ms — paginated list on 500 rows
- [sqlite/POST create] 29,726 req/s, p99 — JSON bind + INSERT, concurrent with maintenance goroutine
- [sqlite/GET integrity] 11,176 req/s, p99 — PRAGMA quick_check on 5500 rows
- [memory] 39 MB RSS after 5500 writes + sustained load
- [race] No data races detected with -race flag on concurrent health + stats + CRUD + maintenance
- [maintenance] 18 optimize ticks during load test (3s interval), 2 auto-checkpoints triggered (50KB WAL threshold)
- [shutdown] Maintenance goroutine stopped cleanly before DB close

Session 19 (migrate revisit, migration dashboard with 5 migrations):
- [migrate/GET status] 29,620 req/s, p99 4.2ms — DB query + checksum computation per migration (5 migrations)
- [migrate/GET version] 36,874 req/s, p99 — single-row MAX query
- [migrate/GET pending] 37,121 req/s, p99 — lightweight applied-versions check
- [migrate/mutation cycles] 100 rapid down/up cycles completed cleanly
- [memory] 25 MB RSS after mutation stress test
- [race] No data races detected with -race flag on concurrent status reads + down/up/redo writes
- [dirty detection] Verified: tampered DB checksum → only affected migration shows dirty=True
- [concurrent redo] Serialized by SQLite write lock; some expected SQL errors, no data races

Session 20 (driver revisit, events app with structured errors + context cancel + memory stats, 5k rows):
- [driver/GET list] 3,303 req/s, p99 40ms — paginated list on 5k rows (COUNT + SELECT)
- [driver/GET single] 55,897 req/s, p99 2.8ms — single read by ID
- [driver/GET stats] 51,001 req/s, p99 3.1ms — Stats() with memory_used + memory_highwater
- [driver/GET error-codes] 22,711 req/s, p99 3.5ms — INSERT + duplicate INSERT + *Error extraction per request
- [driver/slow-query cancel] 746 req/s, p99 41ms — 10M-row CTE interrupted at 10ms timeout via sqlite3_interrupt
- [driver/context-cancel] 50ms timeout correctly interrupts 884ms query; pre-cancelled context returns context.DeadlineExceeded
- [memory] SQLite memory_used: 22 MB, memory_highwater: 23 MB after 5k+ writes and 30k+ load test requests
- [race] No data races detected with -race flag on concurrent CRUD + error extraction + context cancellation + stats

Session 21 (db revisit, blog app with JOINs — 500 posts, 2000 comments, 10 users, 6 tags, post_tags junction):
- [db/INNER JOIN] 13,668 req/s, p99 1.5ms — paginated posts+users JOIN with As() aliasing, COUNT + SELECT
- [db/LEFT JOIN+GROUP BY] 9,652 req/s, p99 2.1ms — post stats with LEFT JOIN comments, COUNT per post
- [db/Comments JOIN] 44,742 req/s, p99 0.6ms — comments+users INNER JOIN, small result set per post
- [db/Multi LEFT JOIN] 4,136 req/s, p99 3.4ms — posts+post_tags+tags double LEFT JOIN, 50 rows
- [db/Author Summary] 32,754 req/s, p99 0.8ms — users LEFT JOIN posts, IfNull, ColEq, GROUP BY, 10 rows
- [db/COALESCE] 46,898 req/s, p99 0.6ms — Coalesce + IfNull + scalar subquery, 10 rows
- [db/EXISTS+JOIN] 14,375 req/s, p99 1.3ms — EXISTS subquery combined with INNER JOIN, 20 rows
- [memory] 38 MB RSS after 70k+ load test requests across all endpoints
- [race] No data races detected with -race flag on concurrent requests across all 7 JOIN endpoints

Session 22 (db revisit, employee analytics with window functions — 200 employees, 5000 sales, 8 departments):
- [db/ROW_NUMBER+RANK+DENSE_RANK] 10,154 req/s, p99 17.5ms — 3 window functions + JOIN, PARTITION BY dept, ORDER BY salary
- [db/SUM OVER+ROWS frame] 432 req/s, p99 441ms — running total with ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW on 5k sales (IO-bound: large window scan)
- [db/LAG+LEAD+LAGDEFAULT] 939 req/s, p99 202ms — 3 offset window functions per row on 5k sales
- [db/NTILE] 13,045 req/s, p99 13.8ms — NTILE(4) quartile assignment + JOIN
- [db/FIRST_VALUE+LAST_VALUE+NTH_VALUE] 3,503 req/s, p99 55.4ms — 3 value functions with ROWS UNBOUNDED PRECEDING TO UNBOUNDED FOLLOWING
- [db/GROUP_CONCAT+LEFT JOIN] 1,214 req/s, p99 158ms — GROUP_CONCAT + GROUP_CONCAT(DISTINCT) with LEFT JOIN + GROUP BY on 200 employees
- [db/dept-summary] 13,796 req/s, p99 12.8ms — GROUP_CONCAT + COUNT + AVG + RANK window, GROUP BY dept (8 rows)
- [db/frames AVG+SUM+MAX] 13,660 req/s, p99 13.0ms — 3 frame types (Preceding(1)/Following(1), Preceding(2)/Following(2), UnboundedPreceding/CurrentRow) on 50 rows
- [memory] 56 MB RSS after 80k+ load test requests, 166 MB peak under sustained concurrent load
- [race] No data races detected with -race flag on concurrent requests across all 8 window+GROUP_CONCAT endpoints

Session 23 (db revisit, sales analytics with set operations + FILTER — 20 products, 2000 orders, 1500 archived orders):
- [db/UNION ALL] 6,705 req/s, p99 27.2ms — combined 3500 rows from 2 tables, ORDER BY total DESC, LIMIT 50
- [db/UNION] 10,937 req/s, p99 15.9ms — deduplicated product IDs across both tables
- [db/EXCEPT] 11,849 req/s, p99 13.1ms — current-only product IDs (8 results)
- [db/INTERSECT] 13,508 req/s, p99 12.6ms — shared product IDs (12 results)
- [db/FILTER aggregate] 19,007 req/s, p99 7.8ms — 8 conditional aggregates (SUM/COUNT × 4 statuses) on 2000 rows
- [db/FILTER+GROUP BY] 15,083 req/s, p99 10.9ms — region breakdown with 5 FILTER aggregates, GROUP BY region (4 rows)
- [db/UNION ALL+FILTER] 20,452 req/s, p99 7.9ms — UNION ALL of 2 pre-aggregated queries with FILTER
- [memory] 40 MB RSS after 35k+ load test requests
- [race] No data races detected with -race flag on concurrent requests across all 7 endpoints

Session 24 (db revisit, org hierarchy with CTEs — 15 departments, 520 employees, 3-level dept tree):
- [db/CTE+JOIN] 24,380 req/s, p99 5.9ms — non-recursive CTE with dept stats aggregate + INNER JOIN
- [db/CTE+window] 9,555 req/s, p99 18.8ms — CTE with ROW_NUMBER window function, top 3 per dept
- [db/multi-CTE] 9,530 req/s, p99 19.1ms — 2 CTEs with LEFT JOINs (emp_counts + salary_stats)
- [db/recursive dept-tree] 12,377 req/s, p99 14.1ms — WITH RECURSIVE 3-level tree traversal with path concatenation
- [db/recursive org-chain] 15,325 req/s, p99 11.0ms — recursive walk up 5-level management chain
- [db/recursive+non-recursive] 9,055 req/s, p99 19.9ms — recursive subtree + non-recursive emp_counts combined
- [db/pure recursive 100] 36,727 req/s, p99 4.0ms — tableless SELECT base case, 100 rows generated
- [db/pure recursive 10k] 1,422 req/s, p99 34.0ms — 10,000 rows generated (IO-bound: JSON serialization)
- [memory] 33 MB RSS after sustained load across all 7 CTE endpoints
- [race] No data races detected with -race flag on concurrent CTE requests

Session 25 (app revisit, health monitor with heartbeat worker, custom health checks, ReadyCh):
- [app/GET health] 82,689 req/s, p99 3.4ms — concurrent CheckHealth (sqlite + custom check), JSON response
- [app/GET ready] 68,034 req/s, p99 2.5ms — Ready() atomic bool check
- [app/GET info] 63,083 req/s, p99 2.5ms — Env() + Version() + Ready() + container.Len()
- [app/GET heartbeats] 20,258 req/s, p99 6.1ms — paginated list with COUNT + SELECT
- [app/POST heartbeats] 31,736 req/s, p99 6.7ms — JSON bind + INSERT
- [memory] 30 MB RSS after 10k+ writes and 50k+ load test requests
- [race] No data races detected with -race flag on concurrent health + CRUD + heartbeat writer
- [ReadyCh] Heartbeat worker confirmed: started only after ReadyCh closed, stopped cleanly on ShuttingDown

Session 26 (container revisit, service registry dashboard with introspection endpoints, 101 tasks):
- [container/GET services] 66,854 req/s, p99 2.4ms — Inspect() with 8 services, new Kind+Caller+ErrorText+JSON tags
- [container/GET inspect-all] 60,420 req/s, p99 2.5ms — Inspect() + Keys() + Health() + Hooks() combined
- [container/GET health] 69,889 req/s, p99 2.5ms — concurrent CheckHealth (sqlite + fake-cache)
- [tasks/GET list] 20,670 req/s, p99 33.2ms — paginated list on 101 rows
- [tasks/POST create] 32,867 req/s, p99 2.5ms — JSON bind + INSERT
- [memory] 46 MB RSS after 200k+ requests, stable (no leak)
- [introspection comparison] Session 13: 59k req/s for Inspect(5 services). Session 26: 67k req/s for Inspect(8 services) + Kind+Caller fields — no regression despite richer ServiceInfo
- [JSON serialization] ServiceStatus MarshalJSON verified at init: "built"/"pending"/"failed" strings, omitempty for empty error

Session 27 (db revisit, settings store with upserts + INSERT...SELECT + RETURNING, 2000+ rows):
- [db/PUT DoUpdateAll+ReturningStar] 30,081 req/s, p99 2.7ms — upsert with DoUpdateAll + partial index WHERE + ReturningStar
- [db/PUT ConflictWhere+Excluded] 32,884 req/s, p99 2.3ms — versioned upsert with ConflictWhere + Excluded() comparison
- [db/POST INSERT...SELECT] 2,552 req/s, p99 15.4ms — archive 2000+ rows via INSERT...SELECT + ON CONFLICT DO NOTHING
- [db/DELETE ReturningStar] 36,677 req/s, p99 2.4ms — soft delete via SoftDelete().ReturningStar()
- [db/GET list] 1,090 req/s, p99 31.1ms — list 2000 settings with COUNT + SELECT (IO-bound full table)
- [memory] 52 MB RSS after 20k+ load test requests across all endpoints
- [race] No data races detected with -race flag on concurrent upserts + archives + deletes + reads
- [version guard] ConflictWhere correctly rejected stale writes: v5 accepted, v3 rejected, v7 accepted
- [idempotent archive] Second INSERT...SELECT returned 0 rows (ON CONFLICT DO NOTHING)

Session 28 (http revisit, bookmark manager with PaginationParams + embedded BindQuery + Created + middleware consolidation, 500 rows):
- [http/GET list] 21,946 req/s, p99 5.6ms — PaginationParams embedded in listRequest, COUNT + SELECT on 500 rows
- [http/GET list-paginated] 28,320 req/s, p99 5.1ms — explicit page=2&per_page=10, smaller result set
- [http/POST create] 29,211 req/s, p99 7.4ms — JSON bind + validation + INSERT, Created() response (201)
- [http/GET single] 62,809 req/s, p99 2.7ms — single read by path param
- [http/POST panic] 34,640 req/s, p99 4.7ms — every request panics, Recover uses consolidated writeErrorJSON
- [memory] 23 MB RSS after 200 writes + 1000 load test reads
- [race] No data races detected with -race build flag
- [pagination] Default: page=1, per_page=20. Clamping: page<=0→1, per_page<=0→20, per_page>100→20
- [embedded BindQuery] PaginationParams + custom Tags field both bound correctly from query string

Session 29 (config revisit, settings inspector with Export + source tracking + sensitive masking + freeze, 500 rows):
- [config/GET export] 65,526 req/s, p99 2.6ms — Export() with 15 keys, source resolution + sensitive masking
- [config/GET frozen] 83,619 req/s, p99 2.1ms — IsFrozen() atomic bool under RLock
- [config/GET keys] 72,425 req/s, p99 2.6ms — Keys() sorted, 15 keys
- [settings/GET list] 21,865 req/s, p99 5.6ms — paginated list on 500 rows
- [settings/POST create] 66,750 req/s, p99 1.1ms — JSON bind + INSERT (conflict returns 409)
- [memory] 37 MB RSS after 500+ writes and 35k+ load test requests
- [race] No data races detected with -race flag on concurrent Export + frozen + keys + settings CRUD
- [source tracking] data_dir → "env" (DATA_DIR set), settings.max_key_length → "env" (SETTINGS_MAX_KEY_LENGTH=512), all others → "default"
- [sensitive masking] api.secret → "***" in Export, original value accessible via Get[string]("api.secret")
- [freeze] POST /freeze-test → panic caught by Recover → 500 with "config: SetDefault called after config is frozen"

Session 30 (app revisit, task queue simulator with PostBoot seeding + PreShutdown drain, 510+ rows):
- [app/GET list] 16,684 req/s, p99 7.4ms — paginated list on 510 rows with background worker running
- [app/GET single] 56,357 req/s, p99 2.9ms — single read by ID
- [app/POST create] 30,222 req/s, p99 2.9ms — JSON bind + INSERT
- [app/GET worker-status] 68,430 req/s, p99 2.5ms — atomic int + bool read
- [app/GET stats] 21,219 req/s, p99 6.1ms — 3 COUNT queries with WHERE filters
- [memory] 33 MB RSS after 510+ writes and 60k+ load test requests
- [race] No data races detected with -race flag on concurrent CRUD + worker processing + PostBoot + PreShutdown
- [PostBoot] tasks module seeded 10 tasks, worker module verified count=10 in its PostBoot — cross-module ordering confirmed
- [PreShutdown] worker drained in-flight work before shutdown; tasks module stopped processing flag — both logged correctly
- [phase timing] startup log: boot_time_ms=9, register_ms=1, framework_ms=4, module_boot_ms=0, post_boot_ms=0, hooks_ms=0

Session 31 (log revisit, event generator with sampling + log query + config introspection, 2500 events, 22k log entries, 7.6MB log file):
- [log/flood 10k] 840,769 msg/s — 10k DEBUG+INFO messages through sampling handler (1:10 DEBUG, 1:5 INFO)
- [log/flood 50k] 1,329,964 msg/s — 50k messages through sampling handler, sustained throughput
- [log/sampling] 10k messages → 1,501 entries on disk (5k DEBUG at 1:10 = 500, 5k INFO at 1:5 = 1000, plus overhead) — correct sampling ratios
- [log/GET query] 94 req/s, p99 270ms — Query with limit=20 on 22k entries (7.6MB JSONL file), IO-bound as expected
- [log/GET query-error] 83 req/s, p99 300ms — Query with ERROR level filter on 22k entries
- [log/GET query-count] 58 req/s, p99 662ms — CountOnly on 22k entries (scans full file, no entries allocated)
- [log/GET query-desc] 27 req/s, p99 742ms — desc order query (collects all, reverses, paginates)
- [log/GET files] 44k req/s, p99 5.1ms — ListFiles directory scan
- [log/GET config] 67k req/s, p99 3.6ms — config.Keys() + Sub() for log.* keys
- [events/GET list] 94k req/s, p99 1.9ms — paginated list on 2k rows (no regression from log changes)
- [events/POST create] 11.5k req/s, p99 9.1ms — JSON bind + INSERT + slog call with sampling active
- [health] 89k req/s, p99 1.8ms — baseline
- [memory] 311 MB RSS after 60k messages + 2.5k events + sustained load test (includes 7.6MB JSONL scan allocations)
- [race] No data races detected with -race flag on concurrent write flood + query + generate + config reads

Session 32 (sqlite+driver revisit, DB admin app with 3 tables — 500 products, 1500 orders, 1000 reviews):
- [sqlite/GET stats] 36,625 req/s, p99 4.4ms — Stats() with CurrentPragmas field (9 PRAGMA reads), no regression from Session 18
- [sqlite/GET table-stats] 3,175 req/s, p99 56.7ms — dbstat JOIN sqlite_master + 3 COUNT(*) on 3 tables
- [sqlite/GET pragmas] 45,342 req/s, p99 3.1ms — Pragmas() map (9 reads: 7 read-pool + 2 write-pool)
- [sqlite/GET pragma] 61,249 req/s, p99 2.6ms — single GetPragma read
- [sqlite/POST set-pragma] 7,803 req/s, p99 2.0ms — SetPragma updates 2 connectors + applies to 10+1 connections
- [sqlite/GET health] 59,762 req/s, p99 2.5ms — no regression from Session 18
- [sqlite/POST checkpoint] 13,871 req/s, p99 0.2ms — sequential, write pool
- [sqlite/GET products] 69,064 req/s, p99 2.6ms — paginated list on 500 rows with connector-initialized connections
- [connector] All 10 read connections verified: cache_size=-16000, mmap_size=268435456, foreign_keys=1, busy_timeout=5000 (all_match=true)
- [set-pragma verify] After SetPragma(cache_size, -32000): all 10 connections updated (verified via verify-connector)
- [memory] 34 MB RSS after sustained load
- [race] No data races detected with -race flag on concurrent Stats+TableStats+Pragmas+SetPragma+CRUD

Session 34 (driver revisit, events app with trace+busy+WAL hooks, 100 hook-test rows + 5100 events):
- [driver/GET list] 18,906 req/s, p99 6.3ms — paginated list on 200 rows, trace hooks active on main DB (DB_TRACE=true)
- [driver/POST create] 29,737 req/s, p99 2.9ms — JSON bind + INSERT, trace + WAL hooks active
- [driver/GET hook-stats] atomic counter reads — sub-ms, no DB overhead
- [driver/trace overhead] reads: -13% (18.9K vs 21.7K req/s), writes: -2.4% (29.7K vs 30.5K req/s) — debug-only, disabled by default
- [driver/direct hooks] 102 trace stmts, 102 trace profiles, 101 WAL commits, 0 busy invocations — all 3 hook types verified via direct Connector test
- [memory] 26.6 MB RSS under sustained load with all hooks active

Session 35 (http+middleware revisit, secure notes app with Chain+SkipPaths+SubjectFromCtx, 5200+ notes):
- [http/GET health] 67,975 req/s, p99 2.4ms — SkipPaths bypasses auth entirely, no JWT overhead
- [http/GET /me] 31,966 req/s, p99 4.4ms — JWT verify + SubjectFromCtx + RoleFromCtx + COUNT query on 5200 rows
- [http/GET list] 5,400 req/s, p99 24.2ms — Chain(Auth, Timeout) + COUNT + SELECT on 5200 rows (IO-bound)
- [http/POST create] 28,224 req/s, p99 3.0ms — Chain(Auth, Timeout) + JSON bind + INSERT
- [http/GET admin-list] 3,223 req/s, p99 40.2ms — Chain(Auth, RequireRole, Timeout) + all-users query on 10K+ rows
- [http/401 reject] 64,610 req/s, p99 2.4ms — Auth middleware short-circuit (no token)
- [http/403 reject] 59,568 req/s, p99 2.5ms — Chain(Auth, RequireRole) short-circuit (wrong role)
- [memory] 54 MB RSS after 100K+ requests with 10K+ rows
- [race] No data races detected with -race flag on all framework tests (28 middleware tests, 11 http tests)

Session 36 (db revisit, banking app with RunTxVal + FindByID + DeleteByID + optimized Exists, 202 accounts):
- [db/GET single FindByID] 59,880 req/s, p99 2.7ms — FindByID[Account] with NotDeleted scope
- [db/POST transfer RunTxVal] 27,413 req/s, p99 1.4ms — 6 DB ops in transaction (2 FindByID + balance check + debit + credit + insert)
- [db/POST exists optimized] 55,048 req/s, p99 2.8ms — SELECT EXISTS(SELECT 1 ... LIMIT 1) duplicate check on 202 rows
- [db/DELETE DeleteByID] 43,744 req/s, p99 5.0ms — hard delete by ID
- [db/GET list] 18,310 req/s, p99 6.5ms — COUNT + paginated SELECT on 202 rows
- [memory] 27 MB RSS after 200+ writes and 30K+ load test requests
- [race] No data races detected with -race flag on concurrent transfers + reads + deletes + exists checks

Session 37 (container revisit, service registry app with 4 modules — 50 services, 100 events, 100 metrics, 9 container services):
- [container/GET Inspect] 65,105 req/s, p99 2.7ms — Inspect() with 9 services + Deps field (no regression from Session 26: 67K with 8 services)
- [container/GET DependencyGraph] 63,486 req/s, p99 2.4ms — DependencyGraph() adjacency list with 3 entries, 6 edges
- [container/GET deps/tree] 64,195 req/s, p99 2.6ms — Inspect() + DependencyGraph() combined per request
- [container/GET stats] 67,884 req/s, p99 2.5ms — Make + Inspect + DependencyGraph + atomic counters combined
- [container/GET keys] 70,690 req/s, p99 2.4ms — Keys() sorted, 9 services
- [container/GET health] 69,637 req/s, p99 2.5ms — CheckHealth with sqlite health check
- [crud/GET services] 10,484 req/s — paginated list on 50 rows (IO-bound baseline)
- [memory] 27 MB RSS after 250+ writes and 60K+ load test requests
- [race] No data races detected with -race flag on concurrent introspection + CRUD + events writes

Session 38 (app revisit, task tracker with 5 modules — admin, tasks, analytics, email (disabled), heartbeat, 6000+ rows):
- [app/GET ModuleInfo] 67,490 req/s, p99 2.7ms — ModuleInfo() with 5 modules (4 active, 1 disabled), tags, deps
- [app/GET health] 66,204 req/s, p99 2.7ms — CheckHealth with sqlite (consistent with Session 37)
- [crud/POST create] 33,712 req/s, p99 2.6ms — JSON decode + INSERT with BaseModel auto-fill
- [analytics/GET stats] 12,780 req/s, p99 10.3ms — 2x COUNT on 6000+ rows
- [memory] 75 MB RSS after 6000+ writes and 40K+ load test requests

Session 39 (config revisit, config admin dashboard API — 23 keys, all described):
- [config/GET export] 45,153 req/s, p99 3.4ms — Export() with 23 keys (description+default+override+source)
- [config/GET keys] 64,772 req/s, p99 2.6ms — Keys() sorted, 23 keys
- [config/GET described] 33,578 req/s, p99 3.8ms — Export + filter described keys (all 23)
- [config/GET overridden] 47,767 req/s, p99 2.9ms — Export + filter overridden keys (1 key)
- [config/GET diff] 42,495 req/s, p99 3.2ms — Diff(boot, current) snapshot comparison
- [config/GET describe] 66,882 req/s, p99 2.5ms — single key detail (Has + GetOr + Description)
- [config/GET sub] 64,671 req/s, p99 2.5ms — Sub("db") with 8 keys
- [memory] 51 MB RSS after 150K+ requests under concurrent load (3x50K parallel)
- [race] No data races detected with -race flag on 7 concurrent endpoint types (35K requests)

Session 40 (log revisit, event logger with consistent sampling + rotation + query early termination, 46K log entries):
- [log/GET list] 86,504 req/s, p99 1.7ms — paginated event list with sampling active on log pipeline
- [log/POST create] 90,449 req/s, p99 1.8ms — JSON bind + INSERT + DEBUG/INFO log entries (sampled)
- [log/sampling-test] 4,651 req/s, p99 7.4ms — 10 requests × 10 log entries each, consistent sampling verified
- [log/GET query-before] 459 req/s, p99 234ms — ascending Query with Before filter (early termination on 14MB/46K-entry file)
- [log/GET query-full] 27.5 req/s, p99 835ms — full scan baseline (same file, no Before filter)
- [log/early-term speedup] 16.7x (459 vs 27.5 req/s) — early termination skips JSON parsing past time boundary
- [memory] 23 MB RSS under sustained load, 61 MB peak
- [race] No data races detected with -race flag on concurrent create + sampling + query + list
- [consistent sampling] Verified: DEBUG 12/50 at rate=5 (24%), INFO 16/50 at rate=3 (32%), 100% all-or-nothing per request_id

Session 41 (migrate revisit, migration admin tool with 3 migrations — articles, comments, tags):
- [migrate/GET status] 37,000 req/s, p99 1.3ms — Status() with 3 applied migrations
- [migrate/GET version] 44,000 req/s, p99 1.0ms — Version() single MAX query
- [migrate/GET validate] 39,000 req/s, p99 1.2ms — Validate() with 3 clean migrations
- [migrate/GET hooks] 70,272 req/s, p99 0.9ms — in-memory hook event list (no DB)
- [migrate/POST concurrent] 10/10 goroutines succeeded, 0 errors — mutex serialization verified
- [migrate/DryRun] validates 3 pending migrations in rolled-back transaction, returns SQL + validity
- [memory] 25 MB RSS under sustained load
- [hooks] 6 events on startup (before+after × 3 migrations), down hooks fire correctly

Session 42 (driver+sqlite revisit, change feed app with update hook + proactive WAL checkpoint, 500+ articles):
- [driver/GET list] 13,500 req/s, p99 9.4ms — paginated list on 500+ rows with update hook active on write pool
- [driver/POST create] 28,414 req/s, p99 7.6ms — JSON bind + INSERT with update hook firing per row (C callback → Go → ring buffer)
- [driver/GET changes] 29,895 req/s, p99 6.2ms — change feed read from 1000-cap ring buffer (RWMutex + snapshot copy)
- [driver/GET stats] 34,297 req/s, p99 4.1ms — DB stats + atomic change counter
- [driver/update-hook overhead] ~7% write throughput reduction (28.4K vs 30.5K baseline), acceptable for row-level notifications
- [driver/WAL checkpoint] proactive checkpoint triggered at wal_size=105,488,512 (100.6 MB), WAL reduced from 70.3 MB to 35.5 MB
- [driver/change tracking] 35,488 total events captured (ring buffer correctly caps at 1000 latest)
- [memory] 39.8 MB RSS after 35K+ writes and 40K+ load test requests
- [race] No data races detected with -race flag on concurrent writes + change feed reads + stats

Session 43 (http revisit, chat room app with SSE Broker + 200 concurrent SSE clients):
- [broker/GET stats] 71,071 req/s, p99 1.1ms — atomic counter reads, no DB, no locks
- [broker/POST message+publish] 21,065 req/s, p99 3.5ms — DB write + SSE publish to topic (20 clients)
- [broker/POST broadcast] 11,208 req/s, p99 3.5ms — fan-out to all 20 connected clients
- [broker/GET list messages] 3,225 req/s, p99 — paginated on 20K+ rows (IO-bound)
- [broker/POST scaled 200 clients] 17,636 req/s, p99 4.0ms — POST+publish with 200 SSE connections (~16% slower than 20)
- [broker/broadcast scaled 200] 1,088 req/s, p99 19.9ms — fan-out to 200 clients per message (200K events/sec effective throughput)
- [memory] 50 MB RSS with 20 SSE clients, 60 MB with 200 SSE clients (only 10 MB growth for 10x connections)
- [race] No data races detected with -race flag on 27 http package tests

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

- [sqlite] Stats() uses the read pool for PRAGMA queries to avoid blocking writes. File sizes via os.Stat are non-fatal (file might be briefly locked during checkpoint). Table/index counts exclude sqlite_* internal tables and _migrations. (Session 11)
- [sqlite] Checkpoint uses TRUNCATE mode (not PASSIVE) — moves all frames AND truncates the WAL file to zero bytes. More aggressive but gives a clean state. Returns busy error if concurrent readers prevent full checkpoint. (Session 11)
- [sqlite] Backup uses VACUUM INTO (not sqlite3_backup API) — produces a defragmented, self-contained copy. Runs on the write pool for consistent snapshot. Destination must not exist to prevent accidental overwrite. Partial files cleaned up on failure. (Session 11)
- [sqlite] PRAGMA mmap_size=256MB added as a default — enables memory-mapped I/O for reads. Doesn't allocate 256MB upfront; it's the maximum mapping size. Safe on 64-bit systems (all target deployments). Configurable via DB_MMAP_SIZE env var. (Session 11)
- [sqlite] Optimize() called automatically in the shutdown hook before Close(). SQLite docs recommend this — analyzes tables with stale statistics so the query planner has fresh data on next startup. Fire-and-forget (error ignored). (Session 11)
- [sqlite] Config defaults registered via SetDefault in Load() for discoverability — config.Keys() and config.All() will include db.* keys. GetOr used with same defaults as fallback for the actual PRAGMA values. (Session 11)
- [sqlite/driver] Blob binding changed from SQLITE_STATIC (nil destructor) to C.CBytes+C.free — matches the existing string binding pattern. SQLITE_STATIC tells SQLite the pointer is permanent, but Go GC can move/collect the backing array. C.CBytes copies to C heap where SQLite safely owns the memory. (Session 11)
- [migrate] Pending() mirrors Status() but returns only unapplied migrations — useful for admin "N pending migrations" display or dry-run before deployment. (Session 11)
- [migrate] Error messages now include statement index "statement 2/5: ..." — helps identify which SQL statement within a migration failed, especially useful for multi-statement migrations. (Session 11)

- [db] RETURNING uses an unexported `returnable` interface with `build() (string, []any, error)` — satisfied by InsertBuilder, UpdateBuilder, DeleteBuilder. Restricts the Returning/ReturningAll generic functions to mutation builders only (not SelectBuilder). API: `db.Returning[T](ctx, q, builder)` and `db.ReturningAll[T](ctx, q, builder)`. (Session 12)
- [db] RETURNING writes unqualified column names (no table prefix) — SQLite's RETURNING clause only accepts bare column names, not "table"."column" qualified references. writeReturning() uses quoteIdent(col.columnName()) not col.WriteSQL(). (Session 12)
- [db] Subquery types (subqueryExpr, inSubqueryExpr, existsExpr) wrap a *SelectBuilder. They call sb.Build() inside WriteSQL() to generate the inner SELECT, then wrap in parentheses. Args are appended in-order — compatible with SQLite's positional parameter binding. (Session 12)
- [db] ExistsSubquery/NotExistsSubquery named to avoid collision with existing Exists() function (which executes a COUNT and returns bool). The subquery versions are expressions (Expr interface) for use in WHERE. (Session 12)
- [db] CaseBuilder supports both searched CASE (Case().When(predicate, result)) and simple CASE (CaseOf(expr).When(value, result)). When/Else accept any — if already an Expr, used as-is; otherwise wrapped in Raw("?", val). This allows mixing column refs and scalar values naturally. (Session 12)
- [db] CaseBuilder implements Expr (not column) — it can appear in SELECT, WHERE, ORDER BY, SET, and anywhere else an expression is accepted. For SELECT with alias, combine with RawColumn or use WriteSQL() manually with fmt.Sprintf. (Session 12)
- [db] toExpr() helper converts any to Expr — shared by CaseBuilder.When() and CaseBuilder.Else(). Simple type switch: Expr passthrough, everything else becomes Raw("?", v). (Session 12)

- [app] *App supplied to container via container.Supply(a) early in run() — modules resolve via container.MustMake[*app.App]() to access Ready() and ShuttingDown(). No circular import: app imports container (already true), and modules import both app and container (already true). Eliminates need to pass App reference explicitly to modules that run background workers. (Session 13)
- [app] Boot/shutdown timing logged at Debug level per module, startup summary at Info level with boot_time, module count, service count, hook count. Shutdown timing also at Info. Gives visibility into slow startups and shutdowns without cluttering normal operation logs. (Session 13)
- [app] Framework service init timing (sqlite, http) logged at Debug level — helps diagnose slow DB opens or migration runs during startup. Uses local time variables (not global state) to avoid any overhead outside the boot path. (Session 13)
- [container] Inspect() acquires per-service locks to read status, never triggers lazy init — safe for admin dashboards to call under load. Sorts by name for stable output. ServiceStatus is a named int type with String() for JSON-friendly serialization. (Session 13)
- [container] Keys() and Len() use RLock on the container, not individual service locks — lightweight for frequent polling (60k+ req/s overhead-free). Keys() returns sorted for deterministic display. (Session 13)
- [container] Hooks() returns a copy of the hooks slice — mutation-safe for iteration in admin endpoints. Same copy pattern as StartHooks/StopHooks internal snapshot. (Session 13)

- [http] SSE lives in framework/http/sse.go as response-level primitives (NewEventStream, SSEWriter) — parallel to response.go (JSON, Error, NoContent). Not a new package — SSE is an HTTP response pattern. The event bus → SSE broker pattern will layer on top when the event bus exists. (Session 14)
- [http] SSE uses http.ResponseController (Go 1.20+) to extend write deadlines per-connection. Each write extends the deadline by 30 seconds, overriding the server's 15s WriteTimeout. This is the modern Go approach — no need to disable server timeouts globally. (Session 14)
- [http] SSE headers: Content-Type: text/event-stream, Cache-Control: no-cache, Connection: keep-alive, X-Accel-Buffering: no (nginx compatibility). All set in NewEventStream before flushing. (Session 14)
- [http] SSEWriter.Done() returns r.Context().Done() — client disconnect detection via context cancellation. The caller's select loop checks this alongside event channels and heartbeat tickers. No framework-owned goroutines — the caller drives the event loop. (Session 14)
- [http] SSEWriter is not goroutine-safe by design — the caller's single event loop is the expected pattern (select with Done/events/heartbeat). This matches Go's ResponseWriter contract. (Session 14)
- [http] SSE event IDs enable client reconnection via Last-Event-ID header. LastEventID(r) extracts it. The framework provides the primitive; replay logic is the caller's responsibility (typically fetching missed events from DB). (Session 14)
- [http] Heartbeat sends SSE comment (": heartbeat\n\n") — invisible to EventSource API but keeps the connection alive through proxies and resets write deadline. Recommended interval: 15 seconds. (Session 14)
- [http] ErrStreamingNotSupported is an APIError sentinel (500) — returned if the ResponseWriter can't flush. In practice this never fires with Sunkern's middleware stack (responseRecorder implements Flush+Unwrap), but guards against broken third-party middleware. (Session 14)

- [middleware] Timeout uses context.WithTimeout (not http.TimeoutHandler) — does NOT buffer the response. The handler and downstream ops (DB, HTTP) that respect context will abort on deadline. Server WriteTimeout is the hard backstop. SSE endpoints should NOT use Timeout. This is the Go-idiomatic approach (gRPC deadlines work the same way). (Session 15)
- [middleware] Password hashing uses PBKDF2-HMAC-SHA256 (RFC 2898), not bcrypt — bcrypt requires golang.org/x/crypto which violates zero-dep rule. PBKDF2 is implementable with stdlib crypto/hmac + crypto/sha256 alone. 600k iterations per OWASP 2023 recommendation. (Session 15)
- [middleware] Password hash format is PHC/Modular Crypt: $pbkdf2-sha256$600000$<base64-salt>$<base64-hash>. Self-describing — the hash carries its own algorithm, iteration count, and salt. CheckPassword extracts parameters from the hash, no external config needed. (Session 15)
- [middleware] Password max length capped at 72 bytes (matching bcrypt's limit). Prevents DoS via extremely long passwords that would take proportionally longer to hash. (Session 15)
- [middleware] APIToken middleware takes a TokenLookup callback — the framework doesn't know how tokens are stored. Service layer provides the lookup (typically: hash token with SHA-256, query DB). Separation of concerns: framework does HTTP plumbing, service owns storage. (Session 15)
- [middleware] APIToken and Auth share the same Claims context key (WithClaims/ClaimsFromCtx). They are mutually exclusive auth methods for the same route group. Downstream handlers don't need to know which auth method was used. (Session 15)
- [middleware] GenerateToken produces 32 random bytes (64 hex chars) — ~256 bits of entropy. Service should store SHA-256(token) in DB, not the plaintext. Token is shown to user once on creation. (Session 15)
- [middleware] extractBearerToken is case-insensitive on "Bearer " prefix per RFC 6750. Auth middleware still uses case-sensitive check (legacy). Minor inconsistency, not worth changing tested code. (Session 15)

- [config] Rule type is `func(key string, value any, exists bool) error` — function type, not interface. Composable (combine Required + OneOf), zero boilerplate for custom rules (just write a function). Same spirit as http.HandlerFunc. (Session 16)
- [config] Validate() snapshots rules under RLock, releases, then calls resolve() per key (which acquires its own RLock). Avoids potential deadlock if a writer waits between two RLock acquisitions on the same goroutine. (Session 16)
- [config] Validate() panics (not returns error) — consistent with config.Load(), Ensure(), and Get[T]. Config validation failures are always fatal — the app should not start with invalid config. Panic message lists ALL violations sorted alphabetically, each with key + env var name + error. (Session 16)
- [config] Rules skip validation when key doesn't exist (return nil) — compose with Required to enforce both existence and constraints. This mirrors HTTP validation: zero-value fields skip non-required rules (Session 6 precedent). (Session 16)
- [config] AddRule accumulates rules (append, not replace) — multiple AddRule calls for the same key all checked. Modules can independently add constraints to shared keys. (Session 16)
- [config] Framework services (log, sqlite, http) do NOT register validation rules — they consume config before Validate() runs in the lifecycle. Framework packages handle their own validation (log panics on bad levels, sqlite uses GetOr fallbacks). Config validation is primarily for service modules whose Boot phase runs AFTER Validate(). (Session 16)
- [config] Validate() placed in app lifecycle after framework init, before module Boot: config.Load() → modules Register() → framework init → config.Validate() → modules Boot(). This means service modules register defaults+rules in Register(), Validate() catches issues, then Boot() reads validated config safely. (Session 16)

- [log] dailyFileWriter uses bufio.Writer (64KB buffer) + background goroutine flushing every 200ms. Batches small writes (~200-500 bytes per log line) into larger file.Write() calls. Trade-off: up to 200ms of data can be lost on crash, but console output (stdout) is the crash-safe path. Periodic flush ensures low-traffic periods don't leave stale buffered data. (Session 17)
- [log] Flush() exported for callers who need to see their own recent log entries on disk (e.g., before querying). Query does NOT auto-flush — explicit is better than implicit, and auto-flushing under load would negate the buffering benefit. Playground handler calls Flush() before Query() to demonstrate the pattern. (Session 17)
- [log] Query desc ordering collects all matching entries in memory then reverses + paginates. Asc ordering streams (keeps only limit entries in memory). Desc is inherently more expensive for large files — acceptable for admin viewer where file sizes are bounded by 7-day retention + daily rotation. (Session 17)
- [log] Query After/Before use time.Time with half-open interval: After <= t < Before. Zero values mean "no bound". Checked before level/search/user_id filters — time range can short-circuit early for sorted files (though currently no early termination since JSONL may not be perfectly ordered). (Session 17)
- [log] CountOnly skips entry allocation (no append to slice) but still parses every JSON line — the bottleneck is JSON parse, not entry allocation. CountOnly saves memory, not CPU. Useful for admin dashboard badges ("42 errors today") without loading entry data. (Session 17)
- [log] flushLoop goroutine stopped via close(done) channel in Close(). Select on done/ticker ensures clean exit. The goroutine is created in newDailyFileWriter and lives until Close(). Container shutdown hook calls Close(), so no leak in production. Tests must call Close() or Flush() before reading file contents. (Session 17)

- [sqlite] maintenance.go is a single background goroutine handling both periodic optimize and WAL checkpoint. One goroutine, one ticker, one done channel. tick() has defer recover() to prevent panics from killing the maintenance loop. (Session 18)
- [sqlite] Maintenance interval configurable via db.optimize_interval (default "1h", time.Duration). Set to "0" to disable. WAL checkpoint threshold via db.wal_checkpoint_threshold (default 100MB). Both registered as config defaults for discoverability. (Session 18)
- [sqlite] WAL auto-checkpoint in maintenance is a backstop for the built-in wal_autocheckpoint PRAGMA. The PRAGMA runs PASSIVE checkpoints (opportunistic, don't reclaim disk space). Maintenance runs TRUNCATE checkpoints (reclaim disk space) only when WAL exceeds the size threshold. They complement each other. (Session 18)
- [sqlite] Health(ctx) pings both pools with context — fast (~7μs), non-blocking. Suitable for liveness/readiness probes. Returns error describing which pool failed (write or read). Does NOT query the database — just verifies connectivity. (Session 18)
- [sqlite] Maintenance lifecycle: created in Load(), started in container OnStart hook, stopped in OnStop hook (before final Optimize + Close). Same hook name "sqlite" — merged start and stop into one hook. Maintenance goroutine always stops before DB closes. (Session 18)
- [sqlite] optimize_interval logged in startup Info message alongside other PRAGMA values. Maintenance start logged at Debug level (not cluttering normal operation). Periodic optimize at Debug, periodic checkpoint at Info (checkpoint is a notable event). (Session 18)

- [migrate] Checksum is SHA-256 of parsed Up+Down statements (not raw file content). Truncated to 128-bit (32 hex chars) — sufficient for change detection, not security. Content-based hashing means whitespace/comment-only changes don't trigger false positives. Up and Down sections separated by null byte in the hash input. (Session 19)
- [migrate] Dirty detection: Status() compares stored checksum vs current file checksum. Empty DB checksum (from pre-tracking era) is NOT considered dirty — graceful upgrade path for existing databases. Only non-empty mismatches are flagged. (Session 19)
- [migrate] execution_ms uses millisecond granularity (time.Since().Milliseconds()). Sub-ms migrations show 0 — this is correct; the unit matches production migration timing expectations. Microseconds would add false precision for a metric that's only meaningful for slow migrations. (Session 19)
- [migrate] ensureTable does CREATE TABLE IF NOT EXISTS with the full schema (including new columns), then two ALTER TABLE ADD COLUMN calls that silently fail on duplicates. This handles both fresh installs and upgrades from the old 3-column schema. No version tracking for the _migrations table itself — the upgrade is idempotent. (Session 19)
- [migrate] applyUp/applyDown split: Up path needs checksum computation + execution timing + INSERT with 4 columns. Down path just executes statements + DELETE. Splitting eliminates the Direction parameter and makes each path self-documenting. The old Direction type is still exported (used by parse.go) but no longer used internally by the engine. (Session 19)
- [migrate] DownTo(version) keeps the target version applied (exclusive lower bound). DownTo(0) rolls back everything — natural extension since version 0 means "no migrations". UpTo(version) is inclusive upper bound — the target version gets applied. (Session 19)
- [migrate] Redo() finds the last applied version via Version(), looks it up in collected migrations, then calls applyDown + applyUp. If the migration file was modified between runs, the new content gets applied and a fresh checksum is recorded — correct behavior for the development iteration use case. (Session 19)
- [migrate] *Engine supplied to container via container.Supply(migrationEngine) in app.go, right after Up() completes. Admin modules resolve via container.MustMake[*migrate.Engine]() to access Status/Pending/Version for dashboard display and Down/Redo for admin-controlled rollbacks. (Session 19)
- [migrate] MigrationStatus has JSON tags — ready for direct serialization in admin API responses. ExecutionMs and AppliedAt use omitempty so unapplied migrations return clean JSON without zero-value noise. (Session 19)

- [driver] Error type uses primary code (lower 8 bits) and extended code (full int). Primary code enables broad matching (CodeConstraint=19), extended enables specific matching (CodeConstraintUnique=2067). sqlite3_extended_result_codes() enabled per connection so sqlite3_extended_errcode() returns detailed sub-codes. (Session 20)
- [driver] Context cancellation uses watchCtx pattern: goroutine watches ctx.Done(), calls sqlite3_interrupt(db) on cancellation. sqlite3_interrupt is documented as thread-safe (callable from different thread/goroutine). For ExecContext, goroutine is stopped immediately after step. For QueryContext, goroutine lives until rows.Close() — necessary because rows.Next() calls sqlite3_step() which may take time. (Session 20)
- [driver] conn.QueryContext creates a prepared statement and returns rows that own the stmt (closeStmt=true). The stmt is finalized when rows are closed. This is the standard pattern for database/sql drivers that implement QueryerContext. (Session 20)
- [driver] rows.Next() checks for SQLITE_INTERRUPT and returns ctx.Err() if the context was cancelled, or the raw SQLite error otherwise. This ensures database/sql receives the expected context error for proper cancellation handling. (Session 20)
- [driver] time.Time binding uses UTC + millisecond format ("2006-01-02 15:04:05.000"). UTC eliminates timezone ambiguity in stored data. Milliseconds match the sub-second precision Go code typically uses. db.scan updated with timeFormatMs fallback — parses both old format (no ms) and new format (with ms). db.FormatTime still writes old format for backward compatibility; driver format is for direct time.Time bindings through database/sql. (Session 20)
- [driver] MemoryUsed() and MemoryHighwater() are global SQLite functions (not per-connection). Exposed in driver package (where CGo lives), consumed by sqlite.Stats(). MemoryHighwater accepts a reset bool — admin dashboard can reset peak tracking. (Session 20)
- [driver] Compile-time interface assertions added: conn implements ExecerContext + QueryerContext, stmt implements StmtExecContext + StmtQueryContext. Catches interface drift at compile time rather than runtime. (Session 20)
- [driver] Nil guards added to conn.Close() and stmt.Close() — safe to call multiple times. conn.Close() captures error message before nilling db pointer. (Session 20)

- [db] As(expr, alias) is a package-level function returning aliasExpr — works with any Expr (columns, aggregates, Raw, Coalesce, etc.). This is preferred over adding As() methods on every column type — one function covers all cases. Scan target structs use `db:"alias_name"` tags to map aliased columns. (Session 21)
- [db] ColEq/ColNe/ColGt/ColLt/ColGte/ColLte are package-level functions taking (Expr, Expr) — solve cross-type column comparisons for JOIN ON conditions. The existing per-type EqCol methods (StringColumn.EqCol(StringColumn)) still work for same-type comparisons. ColEq is the general-purpose alternative. (Session 21)
- [db] Coalesce(exprs...) wraps COALESCE(a, b, ...). IfNull(expr, fallback) wraps IFNULL(expr, fallback) — SQLite's two-argument shorthand. Both return Expr, compose naturally with As() for aliased SELECT columns. Val(v) creates a parameterized literal (Raw("?", v)) — use inside Coalesce/IfNull to inject Go values as SQL parameters. (Session 21)
- [db] Asc(expr)/Desc(expr) are package-level functions returning OrderExpr — allow ordering by aliases, aggregates, or computed expressions that don't have methods. The column-level .Asc()/.Desc() methods remain for the common case. (Session 21)
- [db] CrossJoin(table) has no ON clause. writeJoins() helper extracted from Build() and buildCount() to handle both ON and no-ON cases without duplication. (Session 21)
- [db] JOIN scan pattern: when JOINing tables with overlapping column names (e.g., both have "id"), users MUST use As() to alias and a flat scan-target struct with unique db tags. This is explicit-is-better-than-implicit — no magic prefix stripping or nested struct scanning for JOINs. (Session 21)

- [db] Window functions use package-level functions + WindowDef builder pattern. Over(expr, win) wraps any existing aggregate Expr — no need to modify aggregateExpr or add methods to it. Window-only functions (RowNumber, Rank, etc.) take *WindowDef directly. This follows the established pattern: package-level functions returning Expr, composable with As() for aliasing. (Session 22)
- [db] WindowDef is a struct with exported methods (PartitionBy, OrderBy, Rows/Range/Groups) — builder pattern, not functional options. All parts are optional: OVER() with empty window is valid SQL. WriteSQL renders the window spec including parentheses. (Session 22)
- [db] Frame bounds use a frameBound value type with package-level constructors: UnboundedPreceding (var), CurrentRow (var), UnboundedFollowing (var), Preceding(n) (func), Following(n) (func). Vars for parameterless bounds, funcs for N-valued bounds — zero-ambiguity API. (Session 22)
- [db] Lag/Lead have two variants: Lag(expr, offset, win) returns NULL for missing rows; LagDefault(expr, offset, defaultVal, win) uses a fallback Expr. Separate functions instead of optional parameter — Go doesn't have optional args, and the defaultVal is an Expr (not a simple value), so it's better as a distinct function. (Session 22)
- [db] GroupConcat uses parameterized separator (? placeholder) to prevent SQL injection. GroupConcatDistinct takes no separator — SQLite requires DISTINCT aggregates to have exactly one argument, so GROUP_CONCAT(DISTINCT col, sep) is a syntax error. This is a known SQLite limitation. (Session 22)
- [db] Window function SQL generation uses separate internal types (windowFuncExpr, windowOffsetExpr, windowOffsetDefaultExpr, windowValueExpr, windowNthExpr) rather than one mega-struct. Each type has exactly the fields it needs — no nil checks or mode flags. More types, simpler code per type. (Session 22)

- [db] Query interface — `type Query interface { Build() (string, []any) }` in db.go. Both *SelectBuilder and *SetBuilder satisfy it. QueryAll/QueryOne/QueryVal changed from `*SelectBuilder` to `Query` — backward compatible because *SelectBuilder already has Build(). Count/Exists remain *SelectBuilder-only (they use buildCount() which is SELECT-specific). (Session 23)
- [db] SetBuilder combines multiple Query implementations (not just *SelectBuilder) via the Query interface. This enables nested set operations: `db.Union(db.Intersect(a, b), db.Except(c, d))`. Each sub-query's Build() is called inline, args are concatenated in order. (Session 23)
- [db] Set operation constructors (Union, UnionAll, Intersect, Except) accept variadic `...Query` — works with 2+ queries. EXCEPT typically takes 2, but SQLite supports chaining. OrderBy/Limit/Offset on SetBuilder apply to the combined result set. Column references in ORDER BY must use aliases or positional notation (db.Raw("column_name") or db.Raw("1")) since table-qualified names don't work on combined results. (Session 23)
- [db] Filter(agg, where) wraps any Expr with FILTER (WHERE condition). It's a simple wrapper — no special knowledge of aggregates. This means it composes with any expression, though it only makes semantic sense on aggregates. Consistent with the established pattern: package-level function returning Expr, no modification to existing types. (Session 23)
- [db] Filter composes with Over for conditional window aggregates: `db.Over(db.Filter(db.Sum(col, ""), pred), win)`. SQLite executes FILTER before OVER — the filter restricts which rows contribute to the aggregate within each window partition. (Session 23)

- [db] CTEDef is a struct with name, columns, query, recursive fields. NewCTE/NewRecursiveCTE are package-level constructors — consistent with NewTableInfo, Select, Union patterns. CTEDef is mutable (As() sets body after construction) — necessary for recursive CTEs where the body references the CTE's own Ref(). (Session 24)
- [db] Ref() returns *TableInfo — reuses the existing table reference system. The TableInfo has no registered columns (Star() returns nil), so users must specify explicit Columns() on the SelectBuilder. This is correct: CTE columns are dynamic, not schema-as-code. Ref() caches the TableInfo to avoid repeated allocation. (Session 24)
- [db] Col(name) returns Raw(quoteIdent(cte) + "." + quoteIdent(col)) — a simple Expr without typed methods (no .Eq(), .Gt() etc.). For comparisons, users use ColEq(cte.Col("x"), someCol) or Raw expressions. CTE columns are dynamic; adding typed methods would require a column factory pattern that adds complexity without proportional value. (Session 24)
- [db] writeCTEs() is a shared helper used by all 5 builders. If ANY CTE in the list is recursive, WITH RECURSIVE is used (per SQL standard — the keyword applies to the entire WITH block). Non-recursive CTEs can coexist in a WITH RECURSIVE block. CTEs are comma-separated, each with optional column list. (Session 24)
- [db] Select(nil) produces a FROM-less SELECT — valid SQL needed for CTE base cases (SELECT 1) and scalar expressions (SELECT datetime('now')). Build() and buildCount() guard against nil table: skip FROM clause, skip Star() call. Empty column list with nil table produces invalid SQL, but that's a programmer error caught by the database. (Session 24)
- [db] With() method added to all 5 builders (SelectBuilder, SetBuilder, InsertBuilder, UpdateBuilder, DeleteBuilder) — SQLite supports CTEs with all DML statements. The With clause is prepended in Build() before the main statement. CTE body must implement Query interface (SelectBuilder or SetBuilder). (Session 24)

- [app] Health check system uses HealthChecker interface (Name + Check) + CheckFunc adapter — same pattern as http.Handler/http.HandlerFunc. Interface for complex checkers, function adapter for simple ones. AddHealthCheck is mutex-protected (concurrent module Boot in ModuleGroup). (Session 25)
- [app] CheckHealth runs all checks concurrently with sync.WaitGroup. Each goroutine gets its own slice index — no append data race. Snapshot checkers under RLock before spawning goroutines — safe to add more checkers while a check is running. Status: "healthy" (all pass), "unhealthy" (any fail), "unavailable" (no checkers registered). (Session 25)
- [app] ReadyCh is a channel closed exactly once when app.ready.Store(true). Symmetric with ShuttingDown() — background goroutines select on both to start after boot and stop on shutdown. No Close() method needed — it's a one-time signal. (Session 25)
- [app] Env() resolved from config key "app.env" (APP_ENV env var) after config.Load(), default "development". SetDefault called before Load for discoverability via config.Keys()/All(). Version from WithVersion option only — not from config, since version is a build-time concern. (Session 25)
- [app] Sqlite health check auto-registered in run() right after sunkerndb.Load() — uses CheckFunc adapter wrapping sunkerndb.Global().Health. Framework manages its own health checks; modules add custom ones. (Session 25)
- [app] Startup log uses variadic []any for attrs to conditionally include "version" only when set. Avoids empty version field in JSON output for apps that don't set it. env, health_checks always included. (Session 25)

- [container] ServiceInfo enriched with Kind ("supplied"/"provided") and Caller ("file.go:42"). Kind is derived from the registration method: Provide/Override → "provided", Supply/OverrideSupply → "supplied". Stored on the internal service struct, not computed at query time — Inspect() just copies values. (Session 26)
- [container] Caller captured via runtime.Caller(2) in captureCallerOf(), called from each facade function (Provide/Supply/Override/OverrideSupply). Skip=2: 0=captureCallerOf, 1=facade, 2=external caller. Filename shortened to basename only (last "/" segment) for readability. (Session 26)
- [container] Duplicate-registration panics now include both locations: "duplicate provider for *Svc (registered at app.go:181, duplicate at module.go:15)". The original caller is stored on the service struct; the new caller is captured fresh. This is the #1 debugging aid — instantly tells you where the conflict is. (Session 26)
- [container] HookReport is a value type with Name, DurationMs, Err (string). StartHooks/StopHooks signatures changed from `error` to `([]HookReport, error)`. Only app.go calls these — easy migration. Reports cover only executed hooks (nil callbacks skipped). Rollback reports from StartHooks failure are discarded — only start-phase reports returned. (Session 26)
- [container] ServiceStatus.MarshalJSON added so JSON output uses "pending"/"built"/"failed" strings instead of integer 0/1/2. No UnmarshalJSON — ServiceInfo is output-only (admin API). ServiceInfo.ErrorText is the string representation of Error for JSON; Error field has `json:"-"` tag. (Session 26)
- [container] hookLabel changed to return unquoted names; error messages use %q for consistent quoting. Output: `starting hook "database": connection refused` (same as before for named hooks). Positional fallback: `starting hook "2": ...` (now quoted, was unquoted). Minor cosmetic improvement. (Session 26)
- [container] App.go now logs hook start/stop at Debug level with per-hook name and timing. This provides startup/shutdown observability without polluting normal (Info) output. Example: `hook started hook=http took_ms=0`. (Session 26)

- [db] FromSelect(query Query) on InsertBuilder replaces VALUES with a SELECT subquery. Build() checks `b.fromSelect != nil` and calls query.Build() inline, concatenating args in order. Column list is optional — omitting Columns() produces `INSERT INTO table SELECT ...` which lets SQLite infer columns from SELECT. (Session 27)
- [db] DoUpdateAll() on ConflictBuilder generates SetExcluded for all insert columns EXCEPT conflict targets, "id", and "created_at". Consistent with SetModel() skip logic — primary key and creation timestamp are immutable. Reads cb.insert.columns, so Columns() or Model() must be called before OnConflict(). (Session 27)
- [db] ConflictBuilder.Where() adds WHERE to the ON CONFLICT target clause (partial unique index matching). This is the WHERE between the conflict column list and DO NOTHING/DO UPDATE — tells SQLite which unique index to match. Stored on ConflictBuilder and forwarded to conflictClause.targetWhere in DoNothing/DoUpdate/DoUpdateAll. (Session 27)
- [db] ConflictWhere() on InsertBuilder adds WHERE to the DO UPDATE clause (conditional update). Called AFTER OnConflict().DoUpdate() returns *InsertBuilder. Sets conflictClause.updateWhere — the WHERE after DO UPDATE SET. Use with Excluded() to compare incoming vs existing values (e.g., version-gated upserts). (Session 27)
- [db] Excluded(col) is a package-level function wrapping excludedRef — reuses the existing type used by SetExcluded. Returns Expr, so it works in ColGt/ColEq/And/Or and any expression context. SetExcluded creates a ConflictSet (col = excluded.col); Excluded() creates a bare Expr for use in WHERE conditions. (Session 27)
- [db] Returning(...Expr) replaces Returning(...column) on all mutation builders. Backward compatible: column interface embeds Expr, so all typed columns (StringColumn, IntColumn, etc.) pass through. writeReturning uses type assertion — if expr satisfies column interface, writes unqualified quoteIdent(col.columnName()); otherwise uses expr.WriteSQL(buf, args). This preserves the Session 12 convention of unqualified RETURNING column names. (Session 27)
- [db] ReturningStar() sets returning = []Expr{Raw("*")} — simple, composes with Returning[T]/ReturningAll[T] generic functions unchanged. Raw("*") passes through WriteSQL producing bare `*`. (Session 27)

- [http] PaginationParams is a concrete struct (not interface) with Page/PerPage int fields and query tags. Paginate() is a value method returning (page, perPage, offset) — works on copies, no mutation. DefaultPerPage=20, MaxPerPage=100 are exported constants. Embedding in request structs enables BindQuery to populate pagination fields alongside custom fields. (Session 28)
- [http] BindQuery/BindForm embedded struct recursion mirrors validate.go's parseSpecs pattern — check field.Anonymous && field.Type.Kind() == reflect.Struct, recurse into the embedded value. This enables composable request structs where shared params (pagination, sorting) are embedded alongside endpoint-specific fields. (Session 28)
- [http] Created(w, data) is a thin wrapper over writeJSON(w, 201, envelope{"data": data}). Not a generic "status shorthand factory" — just the one most common status that isn't 200. Adding more (Accepted, etc.) only when playground apps demonstrate repeated need. YAGNI. (Session 28)
- [http] Server timeouts configurable via config keys http.read_timeout, http.write_timeout, http.idle_timeout (time.Duration strings). SetDefault called in NewServer for discoverability via config.Keys()/All(). Default values unchanged (15s/15s/60s) — backward compatible. (Session 28)
- [middleware] writeErrorJSON(w, status, code, msg) is an unexported helper in error.go. Replaces 4 identical inline JSON blocks across auth.go, apitoken.go, recover.go, ratelimit.go. The circular import constraint (middleware can't import parent http) still applies — this is the middleware-internal equivalent of http.Error(). (Session 28)
- [middleware] RateLimit's Retry-After header is set BEFORE writeErrorJSON — writeErrorJSON calls w.WriteHeader which flushes headers. This ordering is correct: set all headers, then write status+body. Previous code set Content-Type manually before WriteHeader, which also worked, but the consolidated helper handles Content-Type internally. (Session 28)

- [config] Source tracking uses resolveWithSource(key) returning (any, Source, bool). resolve() delegates to it and drops the Source — no runtime cost for existing Get/GetOr/Has callers. Source is a named string type ("env"/"file"/"default") for JSON-friendly serialization. (Session 29)
- [config] Export() snapshots sensitive set under RLock, releases, then resolves each key individually (resolveWithSource acquires its own RLock). Same snapshot-then-release pattern as Validate() — avoids holding the lock during env var lookups. Returns []Entry sorted by key — admin API can return this directly. (Session 29)
- [config] MarkSensitive is a registration-phase function (like SetDefault/AddRule), not a per-call flag on Get. This means the sensitive set is fixed after boot — no runtime mutation, consistent with the frozen model. IsSensitive is the read-only accessor. (Session 29)
- [config] Freeze/IsFrozen use the existing sync.RWMutex. Freeze sets frozen=true under write lock. SetDefault/SetDefaults/AddRule/MarkSensitive check frozen under write lock and panic if true. Validate() calls Freeze() at the end — any registration call after Validate is a bug. Load() resets frozen=false for test reuse. (Session 29)
- [config] Environment-specific config files (config.{env}.json) were considered and rejected — Viper research confirmed env vars are the primary override mechanism for container-native deployments. Adding file variants would complicate the mental model without solving a real problem. (Session 29)
- [config] Entry struct has JSON tags on all fields — ready for direct serialization in admin API responses. Value is `any` (not string) to preserve original types from SetDefault (int, bool, duration) in the JSON output. Env source values are always strings (from os.LookupEnv). (Session 29)

- [app] PostBooter is an optional interface (type assertion via `m.(PostBooter)`) — modules not implementing it are silently skipped. No impact on existing modules. Chosen over callback registration (a.OnPostBoot(fn)) because interface approach is discoverable via Go docs and integrates naturally with ModuleGroup delegation. (Session 30)
- [app] PreShutdowner is best-effort (errors collected, not fatal) — differs from PostBooter (fatal on error). Rationale: during shutdown, we want to drain as much as possible before releasing resources. A failing PreShutdown shouldn't prevent other modules from draining. (Session 30)
- [app] PostBoot runs after ALL modules Boot, before container start hooks. If PostBoot fails, a.shutdownModules rolls back ALL booted modules (same as Boot failure). The postBoot() method handles its own rollback internally, consistent with boot(). (Session 30)
- [app] PreShutdown runs before stop hooks and before module Shutdown. Order: close shutdown channel → PreShutdown → stop hooks → module Shutdown. This lets modules drain work while infrastructure is still running (e.g., HTTP server still accepting, DB still available). (Session 30)
- [app] ModuleGroup always implements PostBooter and PreShutdowner (methods exist on the type). When no children implement the interface, the loop is a no-op. This avoids conditional implementation complexity and is consistent with Register/Boot/Shutdown delegation pattern. (Session 30)
- [app] Phase timing uses int64 milliseconds (not time.Duration string) for JSON-friendly output. boot_time_ms is the total, individual phases are additive: register_ms + framework_ms + module_boot_ms + post_boot_ms + hooks_ms ≈ boot_time_ms. Changed from previous boot_time (Duration.String()) to boot_time_ms for consistency. (Session 30)

- [log] samplingHandler uses lock-free atomic.Int64 counters per level — one atomic increment + modulo check per log call. Zero allocation on the sampled-out path. Only DEBUG and INFO are sampled; WARN and ERROR always pass through (they indicate conditions needing attention). (Session 31)
- [log] SamplingRate is a value type (not interface/options) with just Debug/Info int fields. Rates of 0 mean "no sampling". newSamplingHandler returns the inner handler unchanged when all rates are zero — zero indirection overhead when sampling is disabled. (Session 31)
- [log] Sampling config keys: log.sample.debug and log.sample.info (env: LOG_SAMPLE_DEBUG, LOG_SAMPLE_INFO). Default 0 (no sampling). Registered via config.SetDefaults in Load() alongside log.level and log.console.level — all log keys now discoverable via config.Keys(). (Session 31)
- [log] mergedHandler.Enabled now checks `console.Enabled || file.Enabled` instead of always returning true. When both sinks filter a level (e.g., both at WARN, caller logs DEBUG), slog skips Record allocation entirely. Previous behavior always allocated a Record and delegated filtering to Handle. (Session 31)
- [log] Query takes context.Context for cancellation. Checks ctx.Err() every 1024 lines (bitmask: lines&0x3FF==0). This balances cancellation responsiveness against the overhead of the channel check — at ~94 req/s on a 7.6MB file, 1024-line batches add negligible latency but enable mid-scan abort for admin dashboard timeout scenarios. (Session 31)
- [log] QueryResult replaces the ([]Entry, int, error) return. Adds Skipped int for malformed JSONL lines. Admin dashboards can show "3 entries skipped (parse error)" — data quality monitoring without breaking the query. (Session 31)
- [log] Date validation in Query uses regexp `^\d{4}_\d{2}_\d{2}$` — rejects invalid dates early with a clear error message instead of propagating confusing filesystem errors ("open invalid: no such file"). The regex is compiled once (package-level var). (Session 31)
- [log] log.console.level defaults to empty string in SetDefaults, and Load() treats empty string as "inherit from log.level". This is necessary because SetDefaults stores the value, and GetOr finds it (non-missing key), so the old GetOr fallback wouldn't work. Empty string acts as the "not set" sentinel. (Session 31)
- [log] Config defaults registered in Load() (not init()) — consistent with sqlite and http packages. init() runs before config.Load() in app lifecycle, so keys registered there would be lost if config.Load() resets state. Load()-time registration ensures defaults are visible after the config system is initialized. (Session 31)
- [log] samplingHandler.counters is *[2]atomic.Int64 (pointer, not value) — WithAttrs/WithGroup share the same counter pointer so the sampling rate is global across all derived loggers. Copying atomic.Int64 by value would violate sync/atomic contract (must not copy after first use). Shared counters mean slog.With("module", "auth").Debug() and slog.Debug() both contribute to the same DEBUG counter — the 1-in-N rate applies to the total call volume, not per-logger. (Session 31)

- [driver] Connector type implements driver.Connector — sql.OpenDB(connector) replaces sql.Open("sqlite3", dsn). Every new connection from the pool goes through Connect(), which applies all registered PRAGMAs. This is the correct fix for per-connection PRAGMAs in Go's connection pool model (previously only 1 of N read connections got PRAGMAs). (Session 32)
- [driver] Connector.pragmas is a map[string]string (PRAGMA name → full statement). SetPragma updates the map under write lock. Connect() snapshots + sorts keys under read lock for deterministic execution. Sorting is cosmetic (PRAGMA order doesn't matter for SQLite) but helps debugging. (Session 32)
- [driver] SQLITE_ENABLE_DBSTAT_VTAB added to CGo CFLAGS — enables the `dbstat` virtual table for per-table storage introspection. Standard compile flag, safe for production, enables TableStats() to report page-level size metrics per table. (Session 32)
- [sqlite] sql.OpenDB(connector) used for both write and read pools. Shared PRAGMAs (busy_timeout, journal_mode=WAL, synchronous=NORMAL, cache_size, foreign_keys=ON, temp_store=MEMORY, mmap_size) registered on both connectors. Write-only PRAGMAs (journal_size_limit, wal_autocheckpoint) only on write connector. Eliminates the old applyPragmas() function that used Exec on pool (which only hit 1 connection). (Session 32)
- [sqlite] SetPragma updates both connectors (for future connections) AND applies to existing connections. Write pool: single Exec (1 connection). Read pool: grab up to maxConns via db.Conn with 1-second timeout, exec PRAGMA on each, release. In-flight connections get the PRAGMA when recycled via connector. Trade-off: millisecond-level inconsistency for in-use connections is acceptable for admin dashboard use case. (Session 32)
- [sqlite] Pragmas() reads 7 PRAGMAs from read pool + 2 from write pool. Read/write split because wal_autocheckpoint and journal_size_limit are write-connection settings — reading from read pool returns defaults, not the configured values. (Session 32)
- [sqlite] TableStats() joins dbstat with sqlite_master ON type='table' to exclude index btrees. dbstat lists both table and index btrees under the `name` column — without the join, COUNT(*) on an index name causes "no such table" errors. Row counts use COUNT(*) per table (separate query per table, table names from sqlite_master so safe for quoting). (Session 32)
- [sqlite] Stats.CurrentPragmas is map[string]any — populated by Pragmas() during Stats(). Admin dashboard gets complete PRAGMA state alongside file/pool/memory stats in one call. Native types preserved: int64 for integer PRAGMAs, string for text PRAGMAs (journal_mode). (Session 32)

- [migrate] appliedRecords() replaces appliedVersions() — returns map[int]appliedRecord with name, checksum, appliedAt, executionMs. One DB query fetches all columns. Shared by Up, UpTo, Down, DownTo, Pending, HasPending, Status, Validate — previously Status had its own inline query duplicating the logic. (Session 33)
- [migrate] Up()/UpTo() dirty checksum detection: when iterating, already-applied migrations get checksum compared. Empty DB checksum (pre-checksum era) is silently skipped. Only non-empty mismatches emit slog.Warn with version, name, db_checksum, file_checksum. This fires before Validate() — gives immediate feedback during migration phase. (Session 33)
- [migrate] Status() now includes orphaned records. matched map tracks which DB records correspond to files. Unmatched records become MigrationStatus with Orphaned=true, Applied=true, file-derived fields zero-valued. Result re-sorted by version since orphans may interleave with file-based entries. (Session 33)
- [migrate] Validate() is read-only — safe to call from admin endpoints under load. Returns ValidationResult with Orphaned ([]OrphanedMigration) and Dirty ([]DirtyMigration). Clean() helper for quick boolean check. Both slices sorted by version. Types have JSON tags for direct API serialization. (Session 33)
- [migrate] app.go calls Validate() after Up() completes. Logs individual warnings per orphan and per dirty migration. Validate() failure itself is non-fatal (logged as Warn, not returned as error) — startup continues. The double logging (Up warns about dirty, then Validate warns about dirty) is intentional: Up warns during migration phase, Validate provides the structured summary for admin observability. (Session 33)
- [migrate] HasPending() short-circuits on first unapplied migration — O(n) worst case but typically O(1) for up-to-date databases. Uses appliedRecords() which is slightly heavier than needed (fetches all columns), but the simplicity of one shared function outweighs the cost. Migration tables are tiny (tens of rows). (Session 33)

- [driver] CGo callback pattern: hook.c defines C trampolines matching SQLite callback signatures (trace_v2, busy_handler, wal_hook). Trampolines are static, wrapped by non-static install functions (sunkern_install_trace/busy/wal) callable from Go. Go side uses //export for sunkernTraceCallback, sunkernBusyCallback, sunkernWALCallback — these are called by the C trampolines. This two-layer pattern avoids function pointer type casting in Go. (Session 34)
- [driver] cgo.Handle (runtime/cgo) replaces manual handle registry. Each connection with hooks gets a cgo.Handle storing *connHooks (trace, busy, wal callbacks). Handle passed as void* context to C callbacks. cgo.Handle.Delete() called in conn.Close() before sqlite3_close_v2. This is the officially recommended pattern for passing Go values through C callbacks. (Session 34)
- [driver] TraceStmt (mask 0x01) fires at statement start — callback receives original SQL text via X parameter (const char*). TraceProfile (mask 0x02) fires at statement end — callback receives sqlite3_stmt* via P, from which sqlite3_expanded_sql() extracts SQL with bound parameters (must free with sqlite3_free), and sqlite3_int64* via X for nanosecond execution time. Falls back to sqlite3_sql() if expanded_sql returns NULL (BLOB params). (Session 34)
- [driver] BusyFunc receives count (0-based, per locking event). DefaultBusyHandler(maxWait) precomputes max retries: exponential ramp 1→32ms (6 retries = 63ms), then flat 50ms per retry. Total retries = 6 + (maxWait - 63ms) / 50ms. Stateless — safe to share across connections. time.Sleep in the callback is blocking but acceptable (SQLite busy handler is synchronous). (Session 34)
- [driver] WALFunc receives dbName (always "main" for regular databases) and pages (WAL frame count after commit). Runs synchronously in the committing goroutine. Cannot run checkpoint inside callback (would deadlock) — use goroutine/channel signaling for proactive checkpointing. Return value (SQLITE_OK) is currently ignored by SQLite but included for forward compatibility. (Session 34)
- [driver] Connector stores hooks (trace/traceMask/busy/wal) alongside pragmas. Connect() snapshots all under single RLock, applies PRAGMAs first, then installs hooks via conn.installHooks(). If any hooks are set, cgo.NewHandle allocates a handle; otherwise handle stays zero (no overhead for hookless connections). (Session 34)
- [sqlite] db.trace config key (DB_TRACE env var) installs slog.Debug trace on both write and read connectors. Only affects future connections (set before sql.OpenDB). TraceStmt logs original SQL; TraceProfile logs expanded SQL + duration_us. Trace fires at DEBUG level — invisible unless LOG_LEVEL=DEBUG, zero slog overhead at INFO+. Startup log conditionally includes trace=true attribute. (Session 34)
- [driver] Load test overhead: trace adds ~2% write overhead (29.7K vs 30.5K req/s) and ~13% read overhead (18.9K vs 21.7K req/s). Read overhead is higher because reads do COUNT + SELECT (two callbacks per request). Memory: 26.6 MB RSS under load with trace. All within maturity targets. (Session 34)

- [http] Chain() lives in framework/http (not middleware) because the Middleware type is defined there. Chain applies middleware in reverse order (last-added innermost), matching the existing applyMiddleware() and Use() behavior. Zero args returns identity middleware; one arg returns it unchanged. (Session 35)
- [http] SkipIf wraps the middleware at init time (calls mw(next) once) and decides at request time whether to invoke the wrapped handler or the raw next handler. This means the middleware's init-time logic (e.g., Auth's closure over secret) runs once, and per-request overhead is just the skip function call + branch. (Session 35)
- [http] SkipPaths uses map[string]struct{} for O(1) path lookup. Exact path match only — "/health" does not match "/health/deep". This is intentional: prefix matching would accidentally skip auth on unexpected sub-paths. For prefix matching, use SkipIf with strings.HasPrefix. (Session 35)
- [middleware] SubjectFromCtx/RoleFromCtx return "" (not error) when no claims exist. This matches Go's zero-value convention and enables concise inline use: `userID := middleware.SubjectFromCtx(ctx)`. The empty string is always semantically "no user" — handlers that need stricter guarantees should check ClaimsFromCtx != nil. (Session 35)
- [middleware] SubjectFromCtx/RoleFromCtx live in middleware package (not http) because they depend on ClaimsFromCtx and the claimsKey context key, both defined in middleware. Moving them to http would require exporting claimsKey or creating a circular dependency. (Session 35)

- [db] RunTxVal[T] uses named return values (val T, err error) with the same defer-recover pattern as RunTx. On panic: rollback + re-raise. On error: rollback. On success: commit. The generic type parameter means the caller doesn't need to declare a variable outside the closure and assign inside — Go infers T from the function return type. (Session 36)
- [db] Exists() uses SELECT EXISTS(SELECT 1 ... WHERE ... LIMIT 1). SQLite's EXISTS evaluator short-circuits: once the subquery produces a row, it stops scanning. The old COUNT(*) implementation counted ALL matching rows, which was O(n) for broad conditions. The new approach is O(1) for indexed lookups and O(first_match) for scans. The LIMIT 1 inside EXISTS is technically redundant (EXISTS already stops at first row) but makes the intent explicit and acts as a safety belt if the query plan differs. (Session 36)
- [db] FindByID[T] uses newSyntheticColumn for the "id" column to avoid requiring callers to pass a typed column reference. Every Sunkern model has BaseModel with an "id" text column — this is a framework-level assumption that justifies the hardcoded column name. The variadic scopes parameter enables composition (e.g., NotDeleted) without overloading or separate functions. (Session 36)
- [db] DeleteByID is the hard-delete counterpart to SoftDeleteByID. Both use newSyntheticColumn("id") for the WHERE clause. The naming parallel (DeleteByID vs SoftDeleteByID) makes the intent explicit — callers choose between permanent and soft deletion. (Session 36)

- [container] Circular dependency detection checks the per-goroutine resolution stack BEFORE acquiring svc.mu — this prevents the deadlock that would occur if A's provider resolves B, whose provider resolves A (A's mutex is already held). The check uses goID() (runtime.Stack parse) which only runs when len(c.resolving) > 0 — zero overhead on cached hits after boot. (Session 37)
- [container] goID() extracts goroutine ID from runtime.Stack output — a fixed 64-byte stack buffer, parse "goroutine N [...]". Only called during service initialization (not cached hits), so allocation is negligible. The approach is well-established in Go ecosystem (used by golang.org/x/net/trace, testing frameworks). (Session 37)
- [container] Dependency tracking uses a per-goroutine resolveState with stack + deps map. When makeFromContainer is called: (1) recordDep adds the resolved service as a dep of the current top-of-stack (the parent provider), (2) pushResolve adds the name to the stack, (3) provider runs, (4) popResolve pops and returns collected deps as sorted unique []string. deps are stored on the service struct and exposed via Inspect() and DependencyGraph(). (Session 37)
- [container] recordDepIfResolving is the fast-path wrapper: checks len(c.resolving) == 0 under resolveMu and returns immediately if no goroutine is building. This means cached-hit resolution after boot has no goID overhead and only a brief uncontended lock check. (Session 37)
- [container] Lock ordering: c.mu (RLock, released before anything else) → svc.mu (held during provider) → c.resolveMu (briefly acquired/released inside recordDep/pushResolve/popResolve). resolveMu is never held while acquiring svc.mu, preventing deadlocks between tracking and resolution locks. (Session 37)
- [container] DependencyGraph() returns map[string][]string — only includes built services with at least one dep. Each dep list is a copy (mutation-safe). Admin endpoints serialize this directly. The graph represents direct dependencies only (not transitive). (Session 37)
- [container] Container.Reset() is an instance method that clears services, hooks, and resolving map in place. The global Reset() still replaces the pointer (global = New()) for backward compatibility — existing code holding a Global() pointer sees a stale-but-valid container. Instance Reset is for non-global containers in tests. (Session 37)

- [app] Tagger and DependencyDeclarer are optional interfaces (type assertion at call site) — modules not implementing them are silently skipped. Chosen over registration-based approaches (a.AddTags(m, tags)) because interfaces are discoverable via Go docs, type-checked at compile time, and colocated with the module definition. Tags are free-form []string — no predefined taxonomy. (Session 38)
- [app] DependencyDeclarer is validation-only — does NOT reorder modules. Registration order still determines boot order. Rationale: topological sort adds complexity and makes boot order non-obvious. Explicit ordering is Sunkern's philosophy. DependsOn catches misconfiguration (missing/disabled deps), not automates wiring. (Session 38)
- [app] When(bool, Module) uses a bool, not func() bool. The condition is evaluated at Use() time (before Run). Config values from config.json aren't available yet, but env vars are (os.Getenv). This covers the primary use case — container deployments use env vars. For config-file-based conditions, restructure boot order or use env vars. (Session 38)
- [app] disabledModule wraps the inner module and delegates Name() for log/introspection visibility. All lifecycle methods are no-ops. ModuleInfo() explicitly unwraps disabledModule to read Tagger/DependencyDeclarer from the inner module — preserving metadata even when disabled. (Session 38)
- [app] ModuleStatus derived on-the-fly in ModuleInfo() from a.booted set + a.ready flag + isDisabled check. No mutable status map — avoids synchronization complexity. Status is: disabled (When wrapper), registered (not in booted set), booted (in booted set + ready=true), shutdown (in booted set + ready=false). (Session 38)
- [app] Duplicate module name validation runs first in register() before any Register() call. Catches configuration errors early. Disabled modules count toward uniqueness — two modules can't share a name even if one is disabled. (Session 38)
- [app] Disabled modules are excluded from dependency validation in both directions: they don't need their deps validated, and they don't satisfy other modules' deps. If module B depends on A and A is disabled, B gets a clear error — the fix is to also disable B or enable A. No cascading disable. (Session 38)

- [config] Describe() is a registration-phase function (like SetDefault/MarkSensitive), not a per-call annotation. This means descriptions are fixed after boot — consistent with frozen model. Description() is the read-only accessor. Descriptions are stored in a separate map, not on the entry itself — keeps SetDefault API unchanged. (Session 39)
- [config] Entry enriched with DefaultValue, Description, and Overridden fields. DefaultValue is the raw value from SetDefault (preserves original types in JSON: int, bool, duration). Overridden is true when source != SourceDefault AND a default was registered — keys from config.json without a corresponding SetDefault are NOT marked overridden (no default to override). Sensitive DefaultValue is masked to "***" same as Value. (Session 39)
- [config] Diff() is a pure function operating on []Entry slices — no global state, no locks. Uses fmt.Sprintf("%v") for value comparison since raw values can be any type (int, string, bool, etc.). Results sorted by key for deterministic output. Three kinds: "added", "removed", "changed". (Session 39)
- [config] Validate() bug fixed: early-returned when no rules were registered, skipping Freeze(). Now Freeze() is called unconditionally after validation succeeds. Without this fix, SetDefault could be called after Validate() in apps with no rules, which violates the frozen contract. (Session 39)
- [config] app.go SetDefault("app.env") moved from before config.Load() to after. Load() creates a fresh defaults map containing only data_dir, so any SetDefault before Load() was dead code — the default was wiped. The GetOr fallback at line 240 masked this bug. After the fix, the default persists and is visible in config.Keys()/All()/Export(). (Session 39)
- [config] All framework packages now describe their config keys: app (2 keys), db/sqlite (8 keys), http (4 keys), log (4 keys). Module-specific keys are described in module Register(). Total: 18 framework + N module keys, all self-documenting for admin dashboards. (Session 39)

- [log] Consistent per-request sampling uses FNV-1a (inline, no heap allocation) to hash request_id. The hash is deterministic so every log call for the same request gets the same keep/drop decision. When no request_id is in context (e.g., startup, background goroutines), falls back to the existing atomic counter. This means request-scoped logs have complete trace coherence while non-request logs still get volume reduction. (Session 40)
- [log] OnRotate fires callbacks in a single goroutine (sequential, not one goroutine per callback) to avoid goroutine explosion with many callbacks. Callback slice is snapshot-copied under the existing writer mutex (held by caller of rotate), so registering new callbacks during a rotation is safe. prevDate is empty string on first rotation (app startup). (Session 40)
- [log] Query early termination tracks a `pastWindow` boolean. Once an entry's time >= Before, all subsequent entries are skipped without JSON parsing. This relies on JSONL files being chronologically ordered (guaranteed by dailyFileWriter's sequential writes). Only applies to ascending order — descending order must collect all entries anyway for correct pagination. The optimization is especially significant on large files: 16.7x faster on a 14MB file. (Session 40)
- [log] fnv1a iterates bytes (not runes) of the request_id string — safe because request_ids are ASCII (nanoid, UUID). The function avoids hash/fnv package to eliminate heap allocation (hash.Hash32 is an interface, which escapes to heap). Same FNV-1a constants as the stdlib implementation. (Session 40)

- [migrate] sync.Mutex on Engine protects all mutation methods (Up/UpTo/Down/DownTo/Redo/DryRun). In-process lock is correct for Sunkern's single-binary model — no need for DB-level advisory locks. Read-only methods (Status/Validate/Pending/HasPending/Version) don't acquire the lock; SQLite WAL mode handles concurrent reads safely. (Session 41)
- [migrate] Hook registration (OnBeforeEach/OnAfterEach) must happen before Up/Down/etc. — no runtime registration. BeforeEachFunc can return error to abort; AfterEachFunc is fire-and-forget. Hooks fire in registration order, not concurrently. Designed for admin observability and custom logging, not for complex orchestration. (Session 41)
- [migrate] DryRun uses BEGIN + execute all pending Up statements + ROLLBACK. This validates everything — syntax, schema compatibility, FK constraints, index conflicts — not just SQL parsing. On first failure, subsequent migrations are marked "skipped: depends on failed migration" since their validation is meaningless without prior migrations. (Session 41)
- [migrate] Direction type already existed in parse.go; added String() method ("up"/"down") for use in hook callbacks and logging. Hooks receive the concrete Direction value, not a string — caller uses d.String() when needed. (Session 41)
- [migrate] DryRun acquires the engine mutex because it uses the write pool (BeginTx) and concurrent Up() during DryRun would conflict. The write pool has MaxOpenConns=1, so serialization via mutex is the correct approach. (Session 41)

- [driver] sqlite3_update_hook callback signature is `void(void*ctx, int action, const char*db, const char*table, sqlite3_int64 rowid)` — void return, unlike trace (int) and WAL (int). The C trampoline in hook.c matches this signature exactly. UpdateAction constants match the C defines: SQLITE_INSERT=18, SQLITE_DELETE=9, SQLITE_UPDATE=23. (Session 42)
- [driver] UpdateInfo is a value struct (not pointer) — small enough to pass by value (1 int + 2 strings + 1 int64). No heap allocation per callback unless the Go strings escape (C.GoString always allocates). This is the minimal overhead pattern for high-frequency callbacks. (Session 42)
- [driver] SetUpdateHook on Connector follows the same pattern as SetTrace/SetBusyHandler/SetWALHook: set field under write lock, only affects future connections. The sqlite package's DB.SetUpdateHook adds connection recycling (SetMaxIdleConns 0→1) because the write pool has MaxOpenConns=1 and the existing connection needs to be replaced to get the hook. (Session 42)
- [sqlite] DB.SetUpdateHook uses SetMaxIdleConns(0) then SetMaxIdleConns(1) to force idle connection recycling. This is documented database/sql behavior: "If MaxIdleConns is less than the number of idle connections, excess idle connections are closed." The next write creates a fresh connection through the connector. No need for Raw()/ErrBadConn hacks. (Session 42)
- [sqlite] WAL hook installation disables wal_autocheckpoint (SQLite design: sqlite3_wal_hook and sqlite3_wal_autocheckpoint share the same internal slot). This is acceptable because our maintenance goroutine handles checkpointing proactively via the WAL hook signal + periodic tick backstop. The wal_autocheckpoint PRAGMA still shows in `Pragmas()` but reads as 0 when the hook is active. (Session 42)
- [sqlite] Proactive WAL checkpoint uses a buffered channel (cap 1) for coalescing. Under sustained write load, many commits may exceed the threshold simultaneously — only one signal matters because the maintenance goroutine checkpoints the entire WAL. The non-blocking send (`select default`) ensures the hook never blocks the committing goroutine. (Session 42)
- [sqlite] walPageLimit is derived from walCheckpointThreshold (bytes) / page_size at Load time — avoids per-commit division. The WAL hook reports frame count (pages parameter), so comparing int > int is cheaper than stat()ing the WAL file on every commit. (Session 42)
- [sqlite] maintenance.checkpoint() has its own defer/recover separate from tick() — a checkpoint panic shouldn't prevent future periodic optimize calls. Both tick() and the WAL signal path call checkpoint(), so the recovery is needed in both code paths. (Session 42)

- [http] Broker uses single event-loop goroutine for all client/topic map operations — no locks needed. Channel-based register/unregister/messages. Per-client buffered channels (default 16) with non-blocking sends: slow clients get dropped messages, not deadlocks. Atomic counters for Stats() — lock-free reads from any goroutine. (Session 43)
- [http] Broker.Subscribe blocks the calling goroutine (the HTTP handler's goroutine) for the lifetime of the SSE connection. This is the correct pattern: one goroutine per SSE client is natural for Go's net/http model. The event loop goroutine is separate and never blocks on individual clients. (Session 43)
- [http] Broker shutdown: context cancellation closes all client.send channels, then closes b.done. Subscribe loops detect closed channel (msg, ok := <-c.send, ok=false) and return. The defer uses `select { case b.unregister <- c: case <-b.done: }` to avoid blocking if the event loop already exited. No double-close, no goroutine leak. (Session 43)
- [http] Broker.Publish blocks on the messages channel (cap 256). This is intentional backpressure — if the event loop can't keep up, publishers slow down. For non-blocking fire-and-forget, callers wrap Publish in a goroutine. JSON marshaling happens in the calling goroutine, not the event loop. (Session 43)
- [http] Broadcast is Publish with empty topic. The event loop checks `if msg.topic == ""` and iterates all clients instead of a topic subset. Simple, no separate code path. (Session 43)
- [http] Heartbeat uses a sentinel boolean on brokerMessage (heartbeat=true). The Subscribe loop checks this and calls stream.Heartbeat() instead of stream.Send(). This avoids special event names that could collide with user events. (Session 43)
- [http] ResponseRecorder is exported in the http package (not middleware) because middleware can't import the parent http package (circular dependency). The middleware's internal responseRecorder stays separate. The duplication is minimal (30 lines) and necessary for the import direction. (Session 43)
- [http] ResponseRecorder defaults to status 200 (set in NewResponseRecorder). This matches net/http behavior where the first Write implicitly sends 200. Only the first WriteHeader call takes effect — subsequent calls are ignored. (Session 43)

## Next Priorities

<!-- What the last session thinks should come next, in order -->

1. **Revisit `framework/http/middleware`** — last touched Session 35 (8 sessions ago, oldest). Future: CSRF protection (double-submit cookie), conditional middleware by method (SkipMethods), request body caching for retry/inspection
2. **Revisit `framework/db`** — last touched Session 36. Future: batch update helpers (CASE-based multi-row updates), query logging/tracing hook (can now leverage driver trace), prepared statement caching, consider UpdateByID convenience
3. **Revisit `framework/container`** — last touched Session 37. Future: exported container-level generic functions (ProvideToContainer, MustMakeFromContainer) for isolated container testing, provider timeout (context-based deadline for lazy init), service health integration (register health checks from providers automatically)
4. **Revisit `framework/app`** — last touched Session 38. Future: module boot ordering via DependsOn (topological sort — currently validation only), ModuleGroup children introspection (nested ModuleInfo), module enable/disable at runtime via config change callback
5. **Revisit `framework/config`** — last touched Session 39. Future: config watcher (detect env var changes at runtime), config schema generation (JSON Schema from registered keys+rules+descriptions for external tooling), DescribeMany() bulk variant
6. **Revisit `framework/log`** — last touched Session 40. Future: mmap-based query for very large files (deferred), multi-date query (span across day boundaries), log entry struct tags for faster JSON extraction (avoid map[string]any), Query streaming via callback/channel for memory-bounded large result sets
7. **Revisit `framework/sqlite/migrate`** — last touched Session 41. Future: dual-pool support (read pool for Status/Validate/Pending queries, write pool for mutations), migration groups/batches (tag migrations, apply by group), migrate down to named migration (instead of version number)
8. **Revisit `framework/sqlite/driver`** — last touched Session 42. Future: blob I/O (sqlite3_blob_open/read/write/close for incremental large object access), sqlite3_authorizer (per-statement access control), connection-level update hook installation (currently only via Connector, needs pool recycling)
9. **Revisit `framework/sqlite`** — last touched Session 42. Future: expose WAL hook at DB level for custom callbacks (currently only used internally for checkpoint signaling), DB.OnChange(fn) high-level change notification API wrapping update hook with table filtering
10. **Revisit `framework/http`** — last touched Session 43. Future: Broker topic management (add/remove topics after subscribe), Broker event ID + replay buffer for reconnection recovery, Broker per-topic stats, Broker max-clients limit, ResponseRecorder with response body capture option
11. Update sunkern-go-best-practices skill (BLOCKED: need .claude/skills/ write permission)

## In-Progress Work

<!-- If a session timed out, describe what's in the dirty working tree -->

(none)
