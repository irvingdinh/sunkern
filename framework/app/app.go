package app

import (
	"go.uber.org/fx"

	"sunkern.local/framework/config"
	"sunkern.local/framework/http"
)

func New() {
	config.Load()

	fx.New(
		fx.Provide(http.NewServer),
		fx.Invoke(func(_ http.Server) {
			//
		}),
	).Run()
}
