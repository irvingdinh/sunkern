package db

import "strings"

// CaseBuilder builds a SQL CASE expression. It implements Expr so it can be
// used in SELECT columns, WHERE clauses, ORDER BY, SET, and anywhere else
// an expression is accepted.
//
// Two forms are supported:
//
// Searched CASE (no operand — each WHEN has its own condition):
//
//	db.Case().
//	    When(Users.Role.Eq("admin"), "Administrator").
//	    When(Users.Role.Eq("mod"), "Moderator").
//	    Else("User")
//	// → CASE WHEN "users"."role" = ? THEN ? WHEN ... THEN ? ELSE ? END
//
// Simple CASE (compare operand to each WHEN value):
//
//	db.CaseOf(Users.Status).
//	    When(db.Raw("?", "active"), "Active").
//	    When(db.Raw("?", "suspended"), "Suspended").
//	    Else("Unknown")
//	// → CASE "users"."status" WHEN ? THEN ? WHEN ? THEN ? ELSE ? END
type CaseBuilder struct {
	operand Expr // nil for searched CASE
	whens   []whenClause
	elseVal Expr // nil if no ELSE
}

type whenClause struct {
	cond   Expr // condition (searched) or value (simple)
	result Expr
}

// Case starts a searched CASE expression. Each When() receives a boolean
// predicate.
//
//	db.Case().
//	    When(Orders.Total.Gt(1000), "premium").
//	    When(Orders.Total.Gt(100), "standard").
//	    Else("basic")
func Case() *CaseBuilder {
	return &CaseBuilder{}
}

// CaseOf starts a simple CASE expression that compares the given operand
// to each When() value.
//
//	db.CaseOf(Users.Role).
//	    When(db.Raw("?", "admin"), 1).
//	    When(db.Raw("?", "user"), 0).
//	    Else(-1)
func CaseOf(operand Expr) *CaseBuilder {
	return &CaseBuilder{operand: operand}
}

// When adds a WHEN clause. The condition is an Expr (a predicate for searched
// CASE, a value expression for simple CASE). The result can be any scalar
// value or an Expr.
func (b *CaseBuilder) When(cond Expr, result any) *CaseBuilder {
	b.whens = append(b.whens, whenClause{
		cond:   cond,
		result: toExpr(result),
	})
	return b
}

// Else sets the ELSE clause. The value can be any scalar or an Expr.
func (b *CaseBuilder) Else(result any) *CaseBuilder {
	e := toExpr(result)
	b.elseVal = e
	return b
}

// WriteSQL implements Expr. Generates:
//
//	CASE [operand] WHEN cond THEN result [...] [ELSE result] END
func (b *CaseBuilder) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("CASE")

	// Simple CASE: CASE operand WHEN ...
	if b.operand != nil {
		buf.WriteString(" ")
		b.operand.WriteSQL(buf, args)
	}

	for _, w := range b.whens {
		buf.WriteString(" WHEN ")
		w.cond.WriteSQL(buf, args)
		buf.WriteString(" THEN ")
		w.result.WriteSQL(buf, args)
	}

	if b.elseVal != nil {
		buf.WriteString(" ELSE ")
		b.elseVal.WriteSQL(buf, args)
	}

	buf.WriteString(" END")
}

// toExpr converts a value to an Expr. If the value already implements Expr,
// it is returned as-is. Otherwise, it is wrapped in a parameterized Raw("?", val).
func toExpr(v any) Expr {
	if e, ok := v.(Expr); ok {
		return e
	}
	return Raw("?", v)
}
