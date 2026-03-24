package main

import (
	"embed"
	"fmt"
	"io/fs"

	"sunkern.local/framework/app"
	"sunkern.local/service/features/usermod"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func main() {
	migrations, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		panic(fmt.Sprintf("migrations embed: %v", err))
	}

	a := app.New(
		app.WithMigrations(migrations),
	)
	a.Use(usermod.New())
	a.Run()
}
