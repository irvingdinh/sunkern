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
