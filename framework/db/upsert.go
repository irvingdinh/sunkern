package db

import "strings"

// ConflictBuilder is an intermediate builder returned by InsertBuilder.OnConflict.
// Call DoNothing or DoUpdate to complete the ON CONFLICT clause.
type ConflictBuilder struct {
	insert  *InsertBuilder
	targets []column
}

// DoNothing completes the clause with DO NOTHING and returns the InsertBuilder.
func (cb *ConflictBuilder) DoNothing() *InsertBuilder {
	cb.insert.conflict = &conflictClause{
		targets: cb.targets,
		action:  conflictDoNothing,
	}
	return cb.insert
}

// DoUpdate completes the clause with DO UPDATE SET and returns the InsertBuilder.
// Pass one or more ConflictSet values created via Set, SetExcluded, or SetConflictExpr.
func (cb *ConflictBuilder) DoUpdate(sets ...ConflictSet) *InsertBuilder {
	cb.insert.conflict = &conflictClause{
		targets: cb.targets,
		action:  conflictDoUpdate,
		updates: sets,
	}
	return cb.insert
}

// conflictClause holds the ON CONFLICT configuration for InsertBuilder.
type conflictClause struct {
	targets []column
	action  conflictAction
	updates []ConflictSet
}

type conflictAction int

const (
	conflictDoNothing conflictAction = iota
	conflictDoUpdate
)

// ConflictSet represents one "col = expr" in DO UPDATE SET.
type ConflictSet struct {
	col  column
	expr Expr
}

// SetExcluded creates a ConflictSet that assigns the excluded (incoming) value:
//
//	col = "excluded"."col"
func SetExcluded(col column) ConflictSet {
	return ConflictSet{col: col, expr: excludedRef{name: col.columnName()}}
}

// SetConflictExpr creates a ConflictSet with an arbitrary expression.
func SetConflictExpr(col column, expr Expr) ConflictSet {
	return ConflictSet{col: col, expr: expr}
}

// SetConflictVal creates a ConflictSet that assigns a literal parameterized value.
func SetConflictVal(col column, val any) ConflictSet {
	return ConflictSet{col: col, expr: Raw("?", val)}
}

// excludedRef writes "excluded"."column_name" for use in ON CONFLICT DO UPDATE SET.
type excludedRef struct {
	name string
}

func (e excludedRef) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(`"excluded".`)
	buf.WriteString(quoteIdent(e.name))
}

// writeConflict appends the ON CONFLICT clause to buf.
func (c *conflictClause) writeConflict(buf *strings.Builder, args *[]any) {
	buf.WriteString(" ON CONFLICT (")
	for i, col := range c.targets {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(quoteIdent(col.columnName()))
	}
	buf.WriteString(")")

	switch c.action {
	case conflictDoNothing:
		buf.WriteString(" DO NOTHING")
	case conflictDoUpdate:
		buf.WriteString(" DO UPDATE SET ")
		for i, s := range c.updates {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteIdent(s.col.columnName()))
			buf.WriteString(" = ")
			s.expr.WriteSQL(buf, args)
		}
	}
}
