package db

import "strings"

// ConflictBuilder is an intermediate builder returned by InsertBuilder.OnConflict.
// Call DoNothing, DoUpdate, or DoUpdateAll to complete the ON CONFLICT clause.
type ConflictBuilder struct {
	insert  *InsertBuilder
	targets []column
	where   []Expr // WHERE on conflict target (partial unique index)
}

// Where adds a WHERE condition to the ON CONFLICT target clause. This is
// used for partial unique indexes — only rows matching the condition are
// considered for conflict detection.
//
//	db.Insert(&Users.TableInfo).
//	    Model(user).
//	    OnConflict(Users.Email).Where(Users.DeletedAt.IsNull()).
//	    DoUpdate(db.SetExcluded(Users.Name))
func (cb *ConflictBuilder) Where(preds ...Expr) *ConflictBuilder {
	cb.where = append(cb.where, preds...)
	return cb
}

// DoNothing completes the clause with DO NOTHING and returns the InsertBuilder.
func (cb *ConflictBuilder) DoNothing() *InsertBuilder {
	cb.insert.conflict = &conflictClause{
		targets:     cb.targets,
		targetWhere: cb.where,
		action:      conflictDoNothing,
	}
	return cb.insert
}

// DoUpdate completes the clause with DO UPDATE SET and returns the InsertBuilder.
// Pass one or more ConflictSet values created via SetExcluded, SetConflictExpr,
// or SetConflictVal.
func (cb *ConflictBuilder) DoUpdate(sets ...ConflictSet) *InsertBuilder {
	cb.insert.conflict = &conflictClause{
		targets:     cb.targets,
		targetWhere: cb.where,
		action:      conflictDoUpdate,
		updates:     sets,
	}
	return cb.insert
}

// DoUpdateAll completes the clause with DO UPDATE SET for all inserted columns
// except the conflict targets, "id", and "created_at". Each non-excluded column
// is assigned its excluded (incoming) value. This is the most common upsert
// pattern — insert or update all mutable fields.
//
// Columns must already be set via Columns() or Model() before calling.
//
//	db.Insert(&Settings.TableInfo).
//	    Model(setting).
//	    OnConflict(Settings.Key).
//	    DoUpdateAll()
func (cb *ConflictBuilder) DoUpdateAll() *InsertBuilder {
	skip := make(map[string]bool, len(cb.targets)+2)
	skip["id"] = true
	skip["created_at"] = true
	for _, t := range cb.targets {
		skip[t.columnName()] = true
	}

	var sets []ConflictSet
	for _, col := range cb.insert.columns {
		if !skip[col.columnName()] {
			sets = append(sets, SetExcluded(col))
		}
	}

	return cb.DoUpdate(sets...)
}

// conflictClause holds the ON CONFLICT configuration for InsertBuilder.
type conflictClause struct {
	targets     []column
	targetWhere []Expr // WHERE on conflict target (partial unique index)
	action      conflictAction
	updates     []ConflictSet
	updateWhere []Expr // WHERE on DO UPDATE (conditional update)
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

// Excluded returns an expression referencing the excluded (incoming) value
// of a column in an ON CONFLICT DO UPDATE context. Use in WHERE conditions
// or complex SET expressions to compare incoming vs existing values.
//
//	// Only update if incoming version is higher:
//	db.Insert(&Settings.TableInfo).
//	    Model(setting).
//	    OnConflict(Settings.Key).
//	    DoUpdate(db.SetExcluded(Settings.Value), db.SetExcluded(Settings.Version)).
//	    ConflictWhere(db.ColGt(db.Excluded(Settings.Version), Settings.Version))
func Excluded(col column) Expr {
	return excludedRef{name: col.columnName()}
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

	// Partial unique index WHERE
	if len(c.targetWhere) > 0 {
		buf.WriteString(" WHERE ")
		writeExprs(buf, args, c.targetWhere)
	}

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
		// Conditional update WHERE
		if len(c.updateWhere) > 0 {
			buf.WriteString(" WHERE ")
			writeExprs(buf, args, c.updateWhere)
		}
	}
}
