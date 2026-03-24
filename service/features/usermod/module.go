package usermod

import (
	"sunkern.local/framework/app"
	"sunkern.local/framework/container"
	sunkernhttp "sunkern.local/framework/http"
	"sunkern.local/framework/sqlite"
)

// Module is the user module. It registers a simple GET /api/users endpoint
// for testing the full stack.
type Module struct {
	app.BaseModule
}

// New creates a user module.
func New() *Module {
	return &Module{
		BaseModule: app.BaseModule{ModuleName: "users"},
	}
}

// Boot resolves the HTTP server and database from the container and
// registers the /api/users route.
func (m *Module) Boot() error {
	server := container.MustMake[sunkernhttp.Server]()
	db := container.MustMake[*sqlite.DB]()

	h := &handler{db: db.ReadDB()}
	server.Mux().HandleFunc("GET /api/users", h.listUsers)

	return nil
}
