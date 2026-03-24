package http

import (
	"context"
	"errors"
	"log/slog"
	"net"
	httpstd "net/http"
	"time"

	"sunkern.local/framework/config"
	"sunkern.local/framework/container"
	"sunkern.local/framework/http/middleware"
)

// DefaultAddr is the default HTTP listen address.
const DefaultAddr = ":19110"

// Server is the framework's HTTP server abstraction. Modules call Group
// or Mux during Boot to register routes before the listener starts.
type Server interface {
	// Mux returns the underlying ServeMux for direct handler registration.
	Mux() *httpstd.ServeMux

	// Group creates a RouteGroup with the given URL prefix. Routes
	// registered on the group are automatically prefixed and wrapped
	// with the group's middleware stack.
	Group(prefix string) *RouteGroup
}

// NewServer creates an HTTP server, registers lifecycle hooks, and returns
// a Server. It reads the listen address from config (key "http.addr",
// default ":19110"). Intended to be registered via container.Supply.
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
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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

func (s *serverImpl) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		prefix: prefix,
		mux:    s.mux,
	}
}
