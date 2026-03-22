package app

import (
	"go.uber.org/fx"
	"sunkern.local/framework/http"
)

func New() {
	fx.New(
		fx.Provide(http.NewServer),
		fx.Invoke(func(_ http.Server) {
			//
		}),
	).Run()
}
