package users

import (
	"sunkern.local/framework/app"
	"sunkern.local/framework/container"
	sunkernhttp "sunkern.local/framework/http"
	"sunkern.local/framework/sqlite"
)

// Module is the users feature module.
type Module struct {
	app.BaseModule
}

// New creates the users module.
func New() *Module {
	return &Module{
		BaseModule: app.BaseModule{ModuleName: "users"},
	}
}

// Boot resolves dependencies and registers routes.
func (m *Module) Boot() error {
	server := container.MustMake[sunkernhttp.Server]()
	sqliteDB := container.MustMake[*sqlite.DB]()

	uc := &usersController{
		readDB:  sqliteDB.ReadDB(),
		writeDB: sqliteDB.WriteDB(),
	}

	g := server.Group("/api/users")
	g.HandleFunc("GET /", uc.list)

	return nil
}
