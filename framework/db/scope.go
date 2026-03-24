package db

import (
	"context"
	"database/sql"
	"time"
)

// Scope is a reusable query modifier. Define scopes as functions that add
// WHERE conditions, ORDER BY, or other clauses to a SelectBuilder.
type Scope func(*SelectBuilder) *SelectBuilder

// NotDeleted returns a scope that adds WHERE deleted_at IS NULL.
func NotDeleted(col NullTimeColumn) Scope {
	return func(b *SelectBuilder) *SelectBuilder {
		return b.Where(col.IsNull())
	}
}

// Paginate returns a scope that applies LIMIT and OFFSET for simple
// offset-based pagination.
func Paginate(limit, offset int) Scope {
	return func(b *SelectBuilder) *SelectBuilder {
		return b.Limit(limit).Offset(offset)
	}
}

// SoftDelete sets deleted_at and updated_at to NOW() for matching rows.
// It returns an UpdateBuilder — add .Where() and .Exec() to complete it:
//
//	db.SoftDelete(&Users.TableInfo).
//	    Where(Users.ID.Eq(id)).
//	    Exec(ctx, writeDB)
func SoftDelete(table *TableInfo) *UpdateBuilder {
	now := time.Now()
	return Update(table).
		Set(newSyntheticColumn(table.name, "deleted_at"), now).
		Set(newSyntheticColumn(table.name, "updated_at"), now)
}

// Restore clears deleted_at (sets to NULL) and updates updated_at to NOW()
// for matching rows, undoing a soft delete:
//
//	db.Restore(&Users.TableInfo).
//	    Where(Users.ID.Eq(id)).
//	    Exec(ctx, writeDB)
func Restore(table *TableInfo) *UpdateBuilder {
	now := time.Now()
	return Update(table).
		SetExpr(newSyntheticColumn(table.name, "deleted_at"), Raw("NULL")).
		Set(newSyntheticColumn(table.name, "updated_at"), now)
}

// After applies cursor-based forward pagination. When cursor is non-nil,
// it is added as a WHERE condition. Fetches limit+1 rows so the caller can
// detect whether more pages exist using HasMore.
//
// The caller must add an appropriate ORDER BY clause. For forward pagination
// use col.Gt(cursor) with col.Asc(). For reverse use col.Lt(cursor) with
// col.Desc().
//
// Example:
//
//	var cursor Expr
//	if afterID != "" {
//	    cursor = Notes.ID.Gt(afterID)
//	}
//	q := db.Select(&Notes.TableInfo).
//	    OrderBy(Notes.ID.Asc()).
//	    Apply(db.After(cursor, 20))
//	results, _ := db.QueryAll[Note](ctx, readDB, q)
//	items, hasMore := db.HasMore(results, 20)
func After(cursor Expr, limit int) Scope {
	return func(b *SelectBuilder) *SelectBuilder {
		if cursor != nil {
			b = b.Where(cursor)
		}
		return b.Limit(limit + 1)
	}
}

// HasMore splits a result set fetched with limit+1 into the page items and
// a flag indicating whether more items exist beyond this page. Use with the
// After scope.
//
//	results, _ := db.QueryAll[Note](ctx, readDB, q)
//	items, hasMore := db.HasMore(results, 20)
func HasMore[T any](results []T, pageSize int) ([]T, bool) {
	if len(results) > pageSize {
		return results[:pageSize], true
	}
	return results, false
}

// SoftDeleteByID is a convenience that soft-deletes a single row by ID.
func SoftDeleteByID(ctx context.Context, q Querier, table *TableInfo, id string) (sql.Result, error) {
	return SoftDelete(table).
		Where(newComp(newSyntheticColumn(table.name, "id"), "=", id)).
		Exec(ctx, q)
}

// RestoreByID is a convenience that restores a single soft-deleted row by ID.
func RestoreByID(ctx context.Context, q Querier, table *TableInfo, id string) (sql.Result, error) {
	return Restore(table).
		Where(newComp(newSyntheticColumn(table.name, "id"), "=", id)).
		Exec(ctx, q)
}
