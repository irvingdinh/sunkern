package db

import (
	"fmt"
	"strings"
)

// Expr is the fundamental interface for anything that can appear in a SQL
// clause — column references, comparisons, boolean compositions, raw SQL.
type Expr interface {
	WriteSQL(buf *strings.Builder, args *[]any)
}

// ---------------------------------------------------------------------------
// Raw expression escape hatch
// ---------------------------------------------------------------------------

type rawExpr struct {
	sql     string
	rawArgs []any
}

// Raw creates an expression from a literal SQL fragment with ? placeholders.
// Use it for expressions the typed API cannot represent.
func Raw(sql string, args ...any) Expr {
	return rawExpr{sql: sql, rawArgs: args}
}

func (e rawExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.sql)
	*args = append(*args, e.rawArgs...)
}

// ---------------------------------------------------------------------------
// Binary comparison (column op value)
// ---------------------------------------------------------------------------

type compExpr struct {
	col Expr
	op  string
	val any
}

func newComp(col Expr, op string, val any) Expr {
	return compExpr{col: col, op: op, val: val}
}

func (e compExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.col.WriteSQL(buf, args)
	buf.WriteString(" ")
	buf.WriteString(e.op)
	buf.WriteString(" ?")
	*args = append(*args, e.val)
}

// ---------------------------------------------------------------------------
// Column-to-column comparison (col1 op col2)
// ---------------------------------------------------------------------------

type colCompExpr struct {
	left  Expr
	op    string
	right Expr
}

func newColComp(left Expr, op string, right Expr) Expr {
	return colCompExpr{left: left, op: op, right: right}
}

func (e colCompExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.left.WriteSQL(buf, args)
	buf.WriteString(" ")
	buf.WriteString(e.op)
	buf.WriteString(" ")
	e.right.WriteSQL(buf, args)
}

// ---------------------------------------------------------------------------
// IN / NOT IN
// ---------------------------------------------------------------------------

type inExpr struct {
	col    Expr
	values []any
	negate bool
}

func newIn(col Expr, values []any, negate bool) Expr {
	return inExpr{col: col, values: values, negate: negate}
}

func (e inExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.col.WriteSQL(buf, args)
	if e.negate {
		buf.WriteString(" NOT IN (")
	} else {
		buf.WriteString(" IN (")
	}
	for i, v := range e.values {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("?")
		*args = append(*args, v)
	}
	buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// BETWEEN
// ---------------------------------------------------------------------------

type betweenExpr struct {
	col  Expr
	low  any
	high any
}

func newBetween(col Expr, low, high any) Expr {
	return betweenExpr{col: col, low: low, high: high}
}

func (e betweenExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.col.WriteSQL(buf, args)
	buf.WriteString(" BETWEEN ? AND ?")
	*args = append(*args, e.low, e.high)
}

// ---------------------------------------------------------------------------
// IS NULL / IS NOT NULL
// ---------------------------------------------------------------------------

type nullCheckExpr struct {
	col    Expr
	isNull bool
}

func newNullCheck(col Expr, isNull bool) Expr {
	return nullCheckExpr{col: col, isNull: isNull}
}

func (e nullCheckExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.col.WriteSQL(buf, args)
	if e.isNull {
		buf.WriteString(" IS NULL")
	} else {
		buf.WriteString(" IS NOT NULL")
	}
}

// ---------------------------------------------------------------------------
// Boolean composition: AND, OR, NOT
// ---------------------------------------------------------------------------

type andExpr struct {
	children []Expr
}

// And combines predicates with AND. Zero predicates yields nothing,
// one predicate is returned as-is, multiple are wrapped in parentheses.
func And(preds ...Expr) Expr {
	flat := flattenAnd(preds)
	switch len(flat) {
	case 0:
		return Raw("1=1")
	case 1:
		return flat[0]
	default:
		return andExpr{children: flat}
	}
}

func flattenAnd(preds []Expr) []Expr {
	var result []Expr
	for _, p := range preds {
		if a, ok := p.(andExpr); ok {
			result = append(result, a.children...)
		} else {
			result = append(result, p)
		}
	}
	return result
}

func (e andExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("(")
	for i, child := range e.children {
		if i > 0 {
			buf.WriteString(" AND ")
		}
		child.WriteSQL(buf, args)
	}
	buf.WriteString(")")
}

type orExpr struct {
	children []Expr
}

// Or combines predicates with OR. Zero predicates yields nothing,
// one predicate is returned as-is, multiple are wrapped in parentheses.
func Or(preds ...Expr) Expr {
	flat := flattenOr(preds)
	switch len(flat) {
	case 0:
		return Raw("1=0")
	case 1:
		return flat[0]
	default:
		return orExpr{children: flat}
	}
}

func flattenOr(preds []Expr) []Expr {
	var result []Expr
	for _, p := range preds {
		if o, ok := p.(orExpr); ok {
			result = append(result, o.children...)
		} else {
			result = append(result, p)
		}
	}
	return result
}

func (e orExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("(")
	for i, child := range e.children {
		if i > 0 {
			buf.WriteString(" OR ")
		}
		child.WriteSQL(buf, args)
	}
	buf.WriteString(")")
}

type notExpr struct {
	inner Expr
}

// Not negates a predicate: NOT (pred).
func Not(pred Expr) Expr {
	return notExpr{inner: pred}
}

func (e notExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("NOT (")
	e.inner.WriteSQL(buf, args)
	buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// writeExprs joins multiple Expr with AND for WHERE/HAVING clauses.
// ---------------------------------------------------------------------------

func writeExprs(buf *strings.Builder, args *[]any, exprs []Expr) {
	if len(exprs) == 1 {
		exprs[0].WriteSQL(buf, args)
		return
	}
	for i, e := range exprs {
		if i > 0 {
			buf.WriteString(" AND ")
		}
		e.WriteSQL(buf, args)
	}
}

// ---------------------------------------------------------------------------
// quoteIdent quotes a SQL identifier with double quotes.
// ---------------------------------------------------------------------------

func quoteIdent(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}
