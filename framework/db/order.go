package db

import "strings"

// OrderExpr represents an ORDER BY expression: "table"."column" ASC/DESC.
type OrderExpr struct {
	col  Expr
	desc bool
}

// WriteSQL writes the order expression.
func (o OrderExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	o.col.WriteSQL(buf, args)
	if o.desc {
		buf.WriteString(" DESC")
	} else {
		buf.WriteString(" ASC")
	}
}

// Asc creates an ascending ORDER BY expression from any Expr. Use for
// ordering by aliases, aggregates, or other computed expressions.
//
//	db.Select(&Posts.TableInfo).
//	    Columns(db.As(db.CountAll(""), "cnt")).
//	    GroupBy(Posts.AuthorID).
//	    OrderBy(db.Asc(db.Raw(`"cnt"`)))
func Asc(expr Expr) OrderExpr {
	return OrderExpr{col: expr}
}

// Desc creates a descending ORDER BY expression from any Expr.
//
//	db.Select(&Posts.TableInfo).OrderBy(db.Desc(db.Raw(`"comment_count"`)))
func Desc(expr Expr) OrderExpr {
	return OrderExpr{col: expr, desc: true}
}
