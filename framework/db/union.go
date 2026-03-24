package db

import (
	"strconv"
	"strings"
)

// SetBuilder combines multiple SELECT queries with a set operation
// (UNION, UNION ALL, INTERSECT, EXCEPT). The combined result can be ordered
// and paginated with OrderBy, Limit, and Offset.
//
// Create one with Union, UnionAll, Intersect, or Except.
//
//	q := db.Union(
//	    db.Select(&ActiveUsers.TableInfo).Columns(ActiveUsers.Name, ActiveUsers.Email),
//	    db.Select(&ArchivedUsers.TableInfo).Columns(ArchivedUsers.Name, ArchivedUsers.Email),
//	).OrderBy(db.Asc(db.Raw("name"))).Limit(20)
//
//	users, err := db.QueryAll[User](ctx, readDB, q)
type SetBuilder struct {
	op      string // "UNION", "UNION ALL", "INTERSECT", "EXCEPT"
	queries []Query
	orderBy []OrderExpr
	limit   *int
	offset  *int
	ctes    []*CTEDef
}

// Union combines SELECT queries with UNION, which removes duplicate rows
// from the combined result set.
//
//	db.Union(
//	    db.Select(&Customers.TableInfo).Columns(Customers.Email),
//	    db.Select(&Suppliers.TableInfo).Columns(Suppliers.Email),
//	)
func Union(queries ...Query) *SetBuilder {
	return &SetBuilder{op: "UNION", queries: queries}
}

// UnionAll combines SELECT queries with UNION ALL, which keeps all rows
// including duplicates. Faster than Union because it skips deduplication.
//
//	db.UnionAll(
//	    db.Select(&Orders2024.TableInfo).Columns(Orders2024.ID, Orders2024.Total),
//	    db.Select(&Orders2025.TableInfo).Columns(Orders2025.ID, Orders2025.Total),
//	)
func UnionAll(queries ...Query) *SetBuilder {
	return &SetBuilder{op: "UNION ALL", queries: queries}
}

// Intersect combines SELECT queries with INTERSECT, returning only rows
// that appear in ALL result sets.
//
//	db.Intersect(
//	    db.Select(&PremiumUsers.TableInfo).Columns(PremiumUsers.Email),
//	    db.Select(&ActiveUsers.TableInfo).Columns(ActiveUsers.Email),
//	)
func Intersect(queries ...Query) *SetBuilder {
	return &SetBuilder{op: "INTERSECT", queries: queries}
}

// Except combines two SELECT queries with EXCEPT, returning rows from the
// first query that do not appear in the second.
//
//	db.Except(
//	    db.Select(&AllUsers.TableInfo).Columns(AllUsers.Email),
//	    db.Select(&BlockedUsers.TableInfo).Columns(BlockedUsers.Email),
//	)
func Except(queries ...Query) *SetBuilder {
	return &SetBuilder{op: "EXCEPT", queries: queries}
}

// OrderBy appends ORDER BY expressions to the combined result. Column
// references in ORDER BY apply to the result set columns by position or
// alias — use db.Raw("column_name") or db.Asc(db.Raw("1")) for positional.
func (b *SetBuilder) OrderBy(exprs ...OrderExpr) *SetBuilder {
	b.orderBy = append(b.orderBy, exprs...)
	return b
}

// Limit sets the LIMIT clause on the combined result.
func (b *SetBuilder) Limit(n int) *SetBuilder {
	b.limit = &n
	return b
}

// Offset sets the OFFSET clause on the combined result.
func (b *SetBuilder) Offset(n int) *SetBuilder {
	b.offset = &n
	return b
}

// With attaches Common Table Expressions to this set operation. The WITH
// clause is rendered before the first SELECT.
func (b *SetBuilder) With(ctes ...*CTEDef) *SetBuilder {
	b.ctes = append(b.ctes, ctes...)
	return b
}

// Build generates the SQL string and args for the set operation.
func (b *SetBuilder) Build() (string, []any) {
	var buf strings.Builder
	var args []any

	// WITH clause
	writeCTEs(&buf, &args, b.ctes)

	for i, q := range b.queries {
		if i > 0 {
			buf.WriteString(" ")
			buf.WriteString(b.op)
			buf.WriteString(" ")
		}
		sql, qArgs := q.Build()
		buf.WriteString(sql)
		args = append(args, qArgs...)
	}

	// ORDER BY
	if len(b.orderBy) > 0 {
		buf.WriteString(" ORDER BY ")
		for i, o := range b.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			o.WriteSQL(&buf, &args)
		}
	}

	// LIMIT
	if b.limit != nil {
		buf.WriteString(" LIMIT ")
		buf.WriteString(strconv.Itoa(*b.limit))
	}

	// OFFSET
	if b.offset != nil {
		buf.WriteString(" OFFSET ")
		buf.WriteString(strconv.Itoa(*b.offset))
	}

	return buf.String(), args
}
