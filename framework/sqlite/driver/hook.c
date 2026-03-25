#include "sqlite3.h"

// Go callback functions (implemented via //export in hook.go).
extern int sunkernTraceCallback(unsigned int mask, void *ctx, void *p, void *x);
extern int sunkernBusyCallback(void *ctx, int count);
extern int sunkernWALCallback(void *ctx, void *db, const char *name, int pages);
extern void sunkernUpdateCallback(void *ctx, int action, const char *db, const char *table, sqlite3_int64 rowid);

// C trampolines matching the exact signatures SQLite expects as callbacks.

static int _trace_trampoline(unsigned int mask, void *ctx, void *p, void *x) {
	return sunkernTraceCallback(mask, ctx, p, x);
}

static int _busy_trampoline(void *ctx, int count) {
	return sunkernBusyCallback(ctx, count);
}

static int _wal_trampoline(void *ctx, sqlite3 *db, const char *name, int pages) {
	return sunkernWALCallback(ctx, (void *)db, name, pages);
}

static void _update_trampoline(void *ctx, int action, const char *db, const char *table, sqlite3_int64 rowid) {
	sunkernUpdateCallback(ctx, action, db, table, rowid);
}

// Install functions callable from Go — handle function pointer types in C
// so Go never needs to cast function pointers.

void sunkern_install_trace(sqlite3 *db, unsigned int mask, void *ctx) {
	sqlite3_trace_v2(db, mask, _trace_trampoline, ctx);
}

void sunkern_install_busy(sqlite3 *db, void *ctx) {
	sqlite3_busy_handler(db, _busy_trampoline, ctx);
}

void sunkern_install_wal(sqlite3 *db, void *ctx) {
	sqlite3_wal_hook(db, _wal_trampoline, ctx);
}

void sunkern_install_update(sqlite3 *db, void *ctx) {
	sqlite3_update_hook(db, _update_trampoline, ctx);
}
