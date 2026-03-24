package users

import "sunkern.local/framework/db"

// ---------------------------------------------------------------------------
// Domain Model
// ---------------------------------------------------------------------------

// User represents a user account.
type User struct {
	db.BaseModel
	Email string `db:"email" json:"email"`
	Name  string `db:"name"  json:"name"`
}

// ---------------------------------------------------------------------------
// Table Schema
// ---------------------------------------------------------------------------

type usersTable struct {
	db.TableInfo
	ID        db.StringColumn
	Email     db.StringColumn
	Name      db.StringColumn
	CreatedAt db.TimeColumn
	UpdatedAt db.TimeColumn
	DeletedAt db.NullTimeColumn
}

// Users is the typed table reference for the users table.
var Users = func() usersTable {
	t := usersTable{TableInfo: db.NewTableInfo("users")}
	t.ID = db.String(&t.TableInfo, "id")
	t.Email = db.String(&t.TableInfo, "email")
	t.Name = db.String(&t.TableInfo, "name")
	t.CreatedAt = db.Time(&t.TableInfo, "created_at")
	t.UpdatedAt = db.Time(&t.TableInfo, "updated_at")
	t.DeletedAt = db.NullTime(&t.TableInfo, "deleted_at")
	return t
}()
