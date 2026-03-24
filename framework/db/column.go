package db

import (
	"strings"
	"time"
)

// column is a package-level interface for referencing columns by name.
// All typed column types implement this.
type column interface {
	Expr
	columnName() string
	tableName() string
}

// ---------------------------------------------------------------------------
// columnRef writes "table"."column" — shared by all column types.
// ---------------------------------------------------------------------------

type columnRef struct {
	table string
	name  string
}

func (c columnRef) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(quoteIdent(c.table))
	buf.WriteString(".")
	buf.WriteString(quoteIdent(c.name))
}

func (c columnRef) columnName() string { return c.name }
func (c columnRef) tableName() string  { return c.table }

// ===========================================================================
// StringColumn
// ===========================================================================

// StringColumn represents a TEXT column holding a non-null string.
type StringColumn struct{ columnRef }

// String creates a StringColumn and registers it with the table.
func String(t *TableInfo, name string) StringColumn {
	c := StringColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c StringColumn) Eq(val string) Expr        { return newComp(c, "=", val) }
func (c StringColumn) Ne(val string) Expr        { return newComp(c, "<>", val) }
func (c StringColumn) Like(val string) Expr      { return newComp(c, "LIKE", val) }
func (c StringColumn) In(vals ...string) Expr    { return newIn(c, stringsToAny(vals), false) }
func (c StringColumn) NotIn(vals ...string) Expr { return newIn(c, stringsToAny(vals), true) }
func (c StringColumn) IsNull() Expr              { return newNullCheck(c, true) }
func (c StringColumn) IsNotNull() Expr           { return newNullCheck(c, false) }
func (c StringColumn) Asc() OrderExpr            { return OrderExpr{col: c, desc: false} }
func (c StringColumn) Desc() OrderExpr           { return OrderExpr{col: c, desc: true} }
func (c StringColumn) EqCol(other StringColumn) Expr {
	return newColComp(c, "=", other)
}

// ===========================================================================
// IntColumn
// ===========================================================================

// IntColumn represents an INTEGER column.
type IntColumn struct{ columnRef }

// Int creates an IntColumn and registers it with the table.
func Int(t *TableInfo, name string) IntColumn {
	c := IntColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c IntColumn) Eq(val int64) Expr            { return newComp(c, "=", val) }
func (c IntColumn) Ne(val int64) Expr            { return newComp(c, "<>", val) }
func (c IntColumn) Gt(val int64) Expr            { return newComp(c, ">", val) }
func (c IntColumn) Lt(val int64) Expr            { return newComp(c, "<", val) }
func (c IntColumn) Gte(val int64) Expr           { return newComp(c, ">=", val) }
func (c IntColumn) Lte(val int64) Expr           { return newComp(c, "<=", val) }
func (c IntColumn) In(vals ...int64) Expr        { return newIn(c, int64sToAny(vals), false) }
func (c IntColumn) Between(low, high int64) Expr { return newBetween(c, low, high) }
func (c IntColumn) IsNull() Expr                 { return newNullCheck(c, true) }
func (c IntColumn) IsNotNull() Expr              { return newNullCheck(c, false) }
func (c IntColumn) Asc() OrderExpr               { return OrderExpr{col: c, desc: false} }
func (c IntColumn) Desc() OrderExpr              { return OrderExpr{col: c, desc: true} }
func (c IntColumn) EqCol(other IntColumn) Expr   { return newColComp(c, "=", other) }

// ===========================================================================
// FloatColumn
// ===========================================================================

// FloatColumn represents a REAL column.
type FloatColumn struct{ columnRef }

// Float creates a FloatColumn and registers it with the table.
func Float(t *TableInfo, name string) FloatColumn {
	c := FloatColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c FloatColumn) Eq(val float64) Expr            { return newComp(c, "=", val) }
func (c FloatColumn) Ne(val float64) Expr            { return newComp(c, "<>", val) }
func (c FloatColumn) Gt(val float64) Expr            { return newComp(c, ">", val) }
func (c FloatColumn) Lt(val float64) Expr            { return newComp(c, "<", val) }
func (c FloatColumn) Gte(val float64) Expr           { return newComp(c, ">=", val) }
func (c FloatColumn) Lte(val float64) Expr           { return newComp(c, "<=", val) }
func (c FloatColumn) Between(low, high float64) Expr { return newBetween(c, low, high) }
func (c FloatColumn) Asc() OrderExpr                 { return OrderExpr{col: c, desc: false} }
func (c FloatColumn) Desc() OrderExpr                { return OrderExpr{col: c, desc: true} }

// ===========================================================================
// BoolColumn
// ===========================================================================

// BoolColumn represents an INTEGER column storing 0/1 booleans.
type BoolColumn struct{ columnRef }

// Bool creates a BoolColumn and registers it with the table.
func Bool(t *TableInfo, name string) BoolColumn {
	c := BoolColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c BoolColumn) Eq(val bool) Expr { return newComp(c, "=", val) }
func (c BoolColumn) IsTrue() Expr     { return newComp(c, "=", true) }
func (c BoolColumn) IsFalse() Expr    { return newComp(c, "=", false) }
func (c BoolColumn) Asc() OrderExpr   { return OrderExpr{col: c, desc: false} }
func (c BoolColumn) Desc() OrderExpr  { return OrderExpr{col: c, desc: true} }

// ===========================================================================
// TimeColumn
// ===========================================================================

// TimeColumn represents a TEXT column storing timestamps in the canonical
// "2006-01-02 15:04:05" format.
type TimeColumn struct{ columnRef }

// Time creates a TimeColumn and registers it with the table.
func Time(t *TableInfo, name string) TimeColumn {
	c := TimeColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c TimeColumn) Eq(val time.Time) Expr            { return newComp(c, "=", val) }
func (c TimeColumn) Ne(val time.Time) Expr            { return newComp(c, "<>", val) }
func (c TimeColumn) Gt(val time.Time) Expr            { return newComp(c, ">", val) }
func (c TimeColumn) Lt(val time.Time) Expr            { return newComp(c, "<", val) }
func (c TimeColumn) Gte(val time.Time) Expr           { return newComp(c, ">=", val) }
func (c TimeColumn) Lte(val time.Time) Expr           { return newComp(c, "<=", val) }
func (c TimeColumn) Between(low, high time.Time) Expr { return newBetween(c, low, high) }
func (c TimeColumn) IsNull() Expr                     { return newNullCheck(c, true) }
func (c TimeColumn) IsNotNull() Expr                  { return newNullCheck(c, false) }
func (c TimeColumn) Asc() OrderExpr                   { return OrderExpr{col: c, desc: false} }
func (c TimeColumn) Desc() OrderExpr                  { return OrderExpr{col: c, desc: true} }

// ===========================================================================
// NullTimeColumn
// ===========================================================================

// NullTimeColumn represents a nullable TEXT column storing timestamps.
// Scan behavior differs from TimeColumn: it maps to *time.Time in model structs.
type NullTimeColumn struct{ columnRef }

// NullTime creates a NullTimeColumn and registers it with the table.
func NullTime(t *TableInfo, name string) NullTimeColumn {
	c := NullTimeColumn{columnRef{table: t.name, name: name}}
	t.addColumn(c)
	return c
}

func (c NullTimeColumn) Eq(val time.Time) Expr            { return newComp(c, "=", val) }
func (c NullTimeColumn) Ne(val time.Time) Expr            { return newComp(c, "<>", val) }
func (c NullTimeColumn) Gt(val time.Time) Expr            { return newComp(c, ">", val) }
func (c NullTimeColumn) Lt(val time.Time) Expr            { return newComp(c, "<", val) }
func (c NullTimeColumn) Gte(val time.Time) Expr           { return newComp(c, ">=", val) }
func (c NullTimeColumn) Lte(val time.Time) Expr           { return newComp(c, "<=", val) }
func (c NullTimeColumn) Between(low, high time.Time) Expr { return newBetween(c, low, high) }
func (c NullTimeColumn) IsNull() Expr                     { return newNullCheck(c, true) }
func (c NullTimeColumn) IsNotNull() Expr                  { return newNullCheck(c, false) }
func (c NullTimeColumn) Asc() OrderExpr                   { return OrderExpr{col: c, desc: false} }
func (c NullTimeColumn) Desc() OrderExpr                  { return OrderExpr{col: c, desc: true} }

// ===========================================================================
// RawColumn — escape hatch for SELECT expressions
// ===========================================================================

type rawColumn struct {
	expr string
}

// RawColumn returns an expression that writes verbatim SQL in a SELECT clause.
// Use for expressions like "COUNT(*) AS count" or "COALESCE(name, ”) AS name".
func RawColumn(expr string) Expr {
	return rawColumn{expr: expr}
}

func (c rawColumn) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(c.expr)
}

// ===========================================================================
// Helpers
// ===========================================================================

func stringsToAny(vals []string) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}

func int64sToAny(vals []int64) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}
