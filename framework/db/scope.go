package db

// Scope is a reusable query modifier. Define scopes as functions that add
// WHERE conditions, ORDER BY, or other clauses to a SelectBuilder.
type Scope func(*SelectBuilder) *SelectBuilder

// NotDeleted returns a scope that adds WHERE deleted_at IS NULL.
func NotDeleted(col NullTimeColumn) Scope {
	return func(b *SelectBuilder) *SelectBuilder {
		return b.Where(col.IsNull())
	}
}
