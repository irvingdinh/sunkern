package http

import (
	"context"
	"errors"
	"log"
	"net"
	httpstd "net/http"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
)

// Server is the framework's HTTP server abstraction.
type Server interface{}

// NewServer creates an HTTP server, registers lifecycle hooks, and returns
// a Server. It reads the listen address from config (key "http.addr",
// default ":19110"). Intended to be registered via container.Provide.
func NewServer() (Server, error) {
	addr := config.GetOr[string]("http.addr", ":19110")

	mux := httpstd.NewServeMux()
	mux.HandleFunc("/", func(w httpstd.ResponseWriter, r *httpstd.Request) {
		w.WriteHeader(httpstd.StatusOK)
	})

	srv := &httpstd.Server{
		Addr:    addr,
		Handler: mux,
	}

	container.AppendHook(container.Hook{
		OnStart: func(_ context.Context) error {
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

	return &serverImpl{}, nil
}

type serverImpl struct{}
