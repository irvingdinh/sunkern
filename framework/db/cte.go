package db

import "strings"

// CTEDef represents a Common Table Expression (CTE) definition for use in
// WITH clauses. Create one with NewCTE or NewRecursiveCTE.
//
// Non-recursive CTE:
//
//	active := db.NewCTE("active_users",
//	    db.Select(&Users.TableInfo).Where(Users.Active.Eq(true)),
//	)
//	q := db.Select(active.Ref()).With(active).Columns(active.Col("id"), active.Col("name"))
//
// Recursive CTE (create reference first, then set body with As):
//
//	nums := db.NewRecursiveCTE("nums", nil).Columns("n")
//	nums.As(db.UnionAll(
//	    db.Select(nil).Columns(db.Raw("1")),
//	    db.Select(nums.Ref()).Columns(db.Raw("n + 1")).Where(db.Raw("n < ?", 100)),
//	))
//	q := db.Select(nums.Ref()).With(nums).Columns(nums.Col("n"))
type CTEDef struct {
	name      string
	columns   []string // optional explicit column aliases
	query     Query    // CTE body (SELECT, UNION ALL, etc.)
	recursive bool
	ref       *TableInfo // cached table reference
}

// NewCTE creates a non-recursive Common Table Expression.
//
//	cte := db.NewCTE("recent_orders",
//	    db.Select(&Orders.TableInfo).
//	        Where(Orders.CreatedAt.Gt(cutoff)).
//	        OrderBy(Orders.CreatedAt.Desc()).
//	        Limit(100),
//	)
func NewCTE(name string, query Query) *CTEDef {
	return &CTEDef{name: name, query: query}
}

// NewRecursiveCTE creates a recursive Common Table Expression. The body
// typically uses UNION ALL to combine a base case with a recursive case
// that references the CTE itself via Ref().
//
//	tree := db.NewRecursiveCTE("tree", nil).Columns("id", "parent_id", "depth")
//	tree.As(db.UnionAll(
//	    db.Select(&Categories.TableInfo).
//	        Columns(Categories.ID, Categories.ParentID, db.Raw("0")).
//	        Where(Categories.ParentID.IsNull()),
//	    db.Select(&Categories.TableInfo).
//	        Columns(Categories.ID, Categories.ParentID, db.Raw("tree.depth + 1")).
//	        Join(tree.Ref(), db.ColEq(Categories.ParentID, tree.Col("id"))),
//	))
func NewRecursiveCTE(name string, query Query) *CTEDef {
	return &CTEDef{name: name, query: query, recursive: true}
}

// Columns sets explicit column aliases for the CTE definition. These appear
// after the CTE name: WITH name(col1, col2) AS (...).
//
// Column aliases are required for recursive CTEs where the base case uses
// literal values, and optional for non-recursive CTEs (columns are inferred
// from the body query).
func (c *CTEDef) Columns(cols ...string) *CTEDef {
	c.columns = cols
	return c
}

// As sets or replaces the CTE body query. Use this for recursive CTEs where
// the body needs to reference the CTE's own Ref() — create the CTE with a
// nil query first, then call As() after obtaining the reference.
//
//	cte := db.NewRecursiveCTE("counter", nil).Columns("n")
//	cte.As(db.UnionAll(
//	    db.Select(nil).Columns(db.Raw("1")),
//	    db.Select(cte.Ref()).Columns(db.Raw("n + 1")).Where(db.Raw("n < ?", 10)),
//	))
func (c *CTEDef) As(query Query) *CTEDef {
	c.query = query
	return c
}

// Ref returns a *TableInfo representing this CTE for use in Select(), Join(),
// and other clauses that accept a table reference. The returned TableInfo has
// no registered columns — use Columns() on the SelectBuilder or Col() on the
// CTEDef to reference specific columns.
//
//	cte := db.NewCTE("active", query)
//	db.Select(cte.Ref()).With(cte).Columns(cte.Col("id"))
func (c *CTEDef) Ref() *TableInfo {
	if c.ref == nil {
		ti := NewTableInfo(c.name)
		c.ref = &ti
	}
	return c.ref
}

// Col returns an expression referencing a column from this CTE. The expression
// renders as "cte_name"."col_name" and works in SELECT, WHERE, JOIN ON, and
// ORDER BY clauses.
//
//	cte.Col("id")    // → "my_cte"."id"
//	cte.Col("name")  // → "my_cte"."name"
func (c *CTEDef) Col(name string) Expr {
	return Raw(quoteIdent(c.name) + "." + quoteIdent(name))
}

// writeCTEs renders the WITH clause for a list of CTEDefs. If any CTE is
// recursive, WITH RECURSIVE is used (per SQL standard — the keyword applies
// to the entire WITH block, not individual definitions).
func writeCTEs(buf *strings.Builder, args *[]any, ctes []*CTEDef) {
	if len(ctes) == 0 {
		return
	}

	recursive := false
	for _, c := range ctes {
		if c.recursive {
			recursive = true
			break
		}
	}

	if recursive {
		buf.WriteString("WITH RECURSIVE ")
	} else {
		buf.WriteString("WITH ")
	}

	for i, c := range ctes {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteIdent(c.name))

		if len(c.columns) > 0 {
			buf.WriteString("(")
			for j, col := range c.columns {
				if j > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(quoteIdent(col))
			}
			buf.WriteString(")")
		}

		buf.WriteString(" AS (")
		if c.query != nil {
			sql, qArgs := c.query.Build()
			buf.WriteString(sql)
			*args = append(*args, qArgs...)
		}
		buf.WriteString(")")
	}
	buf.WriteString(" ")
}
