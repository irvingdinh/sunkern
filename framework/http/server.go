package http

import (
	"context"
	"errors"
	"log/slog"
	"net"
	httpstd "net/http"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
	"sunkern.local/framework/http/middleware"
)

// DefaultAddr is the default HTTP listen address.
const DefaultAddr = ":19110"

// Server is the framework's HTTP server abstraction. Modules call
// Mux() during Boot to register routes before the listener starts.
type Server interface {
	Mux() *httpstd.ServeMux
}

// NewServer creates an HTTP server, registers lifecycle hooks, and returns
// a Server. It reads the listen address from config (key "http.addr",
// default ":19110"). Intended to be registered via container.Provide.
func NewServer() (Server, error) {
	addr := config.GetOr[string]("http.addr", DefaultAddr)

	mux := httpstd.NewServeMux()
	mux.HandleFunc("/", func(w httpstd.ResponseWriter, r *httpstd.Request) {
		w.WriteHeader(httpstd.StatusOK)
	})

	var handler httpstd.Handler = mux
	handler = middleware.RequestLogger(handler)
	handler = middleware.RequestID(handler)

	srv := &httpstd.Server{
		Addr:    addr,
		Handler: handler,
	}

	container.AppendHook(container.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, httpstd.ErrServerClosed) {
					slog.Error("http: serve error", "error", err)
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	return &serverImpl{mux: mux}, nil
}

type serverImpl struct {
	mux *httpstd.ServeMux
}

func (s *serverImpl) Mux() *httpstd.ServeMux { return s.mux }
