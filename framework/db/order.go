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
