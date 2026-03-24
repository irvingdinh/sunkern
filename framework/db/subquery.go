package db

import "strings"

// ---------------------------------------------------------------------------
// Subquery as expression — wraps a SelectBuilder in parentheses
// ---------------------------------------------------------------------------

type subqueryExpr struct {
	sb *SelectBuilder
}

// Subquery wraps a SelectBuilder as a parenthesized expression. Use in
// WHERE clauses, column lists, or anywhere an Expr is accepted.
//
//	// As a scalar subquery in SELECT:
//	db.Select(&Orders.TableInfo).Columns(
//	    Orders.ID,
//	    db.Subquery(db.Select(&Users.TableInfo).
//	        Columns(Users.Name).
//	        Where(Users.ID.EqCol(Orders.UserID))),
//	)
func Subquery(sb *SelectBuilder) Expr {
	return subqueryExpr{sb: sb}
}

func (e subqueryExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("(")
	sql, subArgs := e.sb.Build()
	buf.WriteString(sql)
	*args = append(*args, subArgs...)
	buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// col IN (SELECT ...) / col NOT IN (SELECT ...)
// ---------------------------------------------------------------------------

type inSubqueryExpr struct {
	col    Expr
	sb     *SelectBuilder
	negate bool
}

// InSubquery creates a "col IN (SELECT ...)" expression.
//
//	// Find users who have placed orders:
//	db.Select(&Users.TableInfo).Where(
//	    db.InSubquery(Users.ID,
//	        db.Select(&Orders.TableInfo).Columns(Orders.UserID)),
//	)
func InSubquery(col Expr, sb *SelectBuilder) Expr {
	return inSubqueryExpr{col: col, sb: sb, negate: false}
}

// NotInSubquery creates a "col NOT IN (SELECT ...)" expression.
//
//	// Find users who have NOT placed orders:
//	db.Select(&Users.TableInfo).Where(
//	    db.NotInSubquery(Users.ID,
//	        db.Select(&Orders.TableInfo).Columns(Orders.UserID)),
//	)
func NotInSubquery(col Expr, sb *SelectBuilder) Expr {
	return inSubqueryExpr{col: col, sb: sb, negate: true}
}

func (e inSubqueryExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.col.WriteSQL(buf, args)
	if e.negate {
		buf.WriteString(" NOT IN (")
	} else {
		buf.WriteString(" IN (")
	}
	sql, subArgs := e.sb.Build()
	buf.WriteString(sql)
	*args = append(*args, subArgs...)
	buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// EXISTS (SELECT ...) / NOT EXISTS (SELECT ...)
// ---------------------------------------------------------------------------

type existsExpr struct {
	sb     *SelectBuilder
	negate bool
}

// ExistsSubquery creates an "EXISTS (SELECT ...)" predicate. Use in WHERE
// clauses for correlated subquery checks.
//
//	// Find users who have at least one order:
//	db.Select(&Users.TableInfo).Where(
//	    db.ExistsSubquery(
//	        db.Select(&Orders.TableInfo).
//	            Columns(db.Raw("1")).
//	            Where(Orders.UserID.EqCol(Users.ID))),
//	)
func ExistsSubquery(sb *SelectBuilder) Expr {
	return existsExpr{sb: sb, negate: false}
}

// NotExistsSubquery creates a "NOT EXISTS (SELECT ...)" predicate.
//
//	// Find users with no orders:
//	db.Select(&Users.TableInfo).Where(
//	    db.NotExistsSubquery(
//	        db.Select(&Orders.TableInfo).
//	            Columns(db.Raw("1")).
//	            Where(Orders.UserID.EqCol(Users.ID))),
//	)
func NotExistsSubquery(sb *SelectBuilder) Expr {
	return existsExpr{sb: sb, negate: true}
}

func (e existsExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	if e.negate {
		buf.WriteString("NOT EXISTS (")
	} else {
		buf.WriteString("EXISTS (")
	}
	sql, subArgs := e.sb.Build()
	buf.WriteString(sql)
	*args = append(*args, subArgs...)
	buf.WriteString(")")
}
