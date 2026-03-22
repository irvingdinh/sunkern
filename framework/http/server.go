package http

import (
	"context"
	"errors"
	"log"
	"net"
	httpstd "net/http"

	"go.uber.org/fx"
)

type Server interface{}

func NewServer(lc fx.Lifecycle) Server {
	mux := httpstd.NewServeMux()
	mux.HandleFunc("/", func(w httpstd.ResponseWriter, r *httpstd.Request) {
		w.WriteHeader(httpstd.StatusOK)
	})

	srv := &httpstd.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, httpstd.ErrServerClosed) {
					log.Panic(err.Error())
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	return &serverImpl{}
}

type serverImpl struct{}
