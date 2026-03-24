package db

import "strings"

// aggregateExpr writes "FUNC(expr) AS alias" or "FUNC(*) AS alias".
type aggregateExpr struct {
	fn    string // SQL function name: COUNT, SUM, AVG, MIN, MAX
	inner Expr   // nil means * (for COUNT)
	alias string
}

func (e aggregateExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.fn)
	buf.WriteString("(")
	if e.inner != nil {
		e.inner.WriteSQL(buf, args)
	} else {
		buf.WriteString("*")
	}
	buf.WriteString(")")
	if e.alias != "" {
		buf.WriteString(" AS ")
		buf.WriteString(quoteIdent(e.alias))
	}
}

// CountAll returns COUNT(*) with the given alias.
func CountAll(alias string) Expr {
	return aggregateExpr{fn: "COUNT", alias: alias}
}

// CountCol returns COUNT(column) with the given alias.
func CountCol(col Expr, alias string) Expr {
	return aggregateExpr{fn: "COUNT", inner: col, alias: alias}
}

// Sum returns SUM(column) with the given alias.
func Sum(col Expr, alias string) Expr {
	return aggregateExpr{fn: "SUM", inner: col, alias: alias}
}

// Avg returns AVG(column) with the given alias.
func Avg(col Expr, alias string) Expr {
	return aggregateExpr{fn: "AVG", inner: col, alias: alias}
}

// Min returns MIN(column) with the given alias.
func Min(col Expr, alias string) Expr {
	return aggregateExpr{fn: "MIN", inner: col, alias: alias}
}

// Max returns MAX(column) with the given alias.
func Max(col Expr, alias string) Expr {
	return aggregateExpr{fn: "MAX", inner: col, alias: alias}
}

// ---------------------------------------------------------------------------
// GROUP_CONCAT — SQLite string aggregation
// ---------------------------------------------------------------------------

type groupConcatExpr struct {
	inner     Expr
	separator string // empty means default (comma)
	distinct  bool
}

func (e groupConcatExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("GROUP_CONCAT(")
	if e.distinct {
		// SQLite requires DISTINCT aggregates to have exactly one argument.
		// Separator cannot be combined with DISTINCT — always uses comma.
		buf.WriteString("DISTINCT ")
		e.inner.WriteSQL(buf, args)
	} else {
		e.inner.WriteSQL(buf, args)
		if e.separator != "" {
			buf.WriteString(", ?")
			*args = append(*args, e.separator)
		}
	}
	buf.WriteString(")")
}

// GroupConcat returns GROUP_CONCAT(col, separator). Concatenates all non-NULL
// values of col into a single string, separated by separator. Pass an empty
// separator to use SQLite's default (comma).
//
//	// Comma-separated tag names per post:
//	db.As(db.GroupConcat(Tags.Name, ","), "tag_names")
//
//	// Pipe-separated:
//	db.As(db.GroupConcat(Tags.Name, " | "), "tag_list")
func GroupConcat(col Expr, separator string) Expr {
	return groupConcatExpr{inner: col, separator: separator}
}

// GroupConcatDistinct returns GROUP_CONCAT(DISTINCT col). Eliminates duplicate
// values before concatenation. SQLite requires DISTINCT aggregates to have
// exactly one argument, so the separator is always comma (SQLite default).
//
//	db.As(db.GroupConcatDistinct(Tags.Name), "unique_tags")
func GroupConcatDistinct(col Expr) Expr {
	return groupConcatExpr{inner: col, distinct: true}
}
