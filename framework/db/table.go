package db

import "strings"

// TableInfo is the base type embedded in all table schema structs. It holds
// the SQL table name and a list of all columns (populated by column
// constructors like String, Int, Time, etc.).
type TableInfo struct {
	name    string
	columns []Expr // columns in declaration order
}

// NewTableInfo creates a TableInfo with the given SQL table name.
func NewTableInfo(name string) TableInfo {
	return TableInfo{name: name}
}

// TableName returns the SQL table name.
func (t *TableInfo) TableName() string { return t.name }

// Star returns all registered columns in declaration order. Use it with
// Select to select every column: db.Select(&Users.TableInfo).
func (t *TableInfo) Star() []Expr { return t.columns }

// addColumn appends a column to the table's column list. Called by each
// column constructor (String, Int, Time, etc.).
func (t *TableInfo) addColumn(col Expr) {
	t.columns = append(t.columns, col)
}

// WriteSQL writes the quoted table name: "users".
func (t *TableInfo) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(quoteIdent(t.name))
}
