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

// Default timeout values. Overridable via config keys http.read_timeout,
// http.write_timeout, and http.idle_timeout (as time.Duration strings).
const (
	DefaultReadTimeout  = 15 * time.Second
	DefaultWriteTimeout = 15 * time.Second
	DefaultIdleTimeout  = 60 * time.Second
)

// NewServer creates an HTTP server, registers lifecycle hooks, and returns
// a Server. It reads configuration from:
//   - http.addr — listen address (default ":19110")
//   - http.read_timeout — read timeout (default 15s)
//   - http.write_timeout — write timeout (default 15s)
//   - http.idle_timeout — idle timeout (default 60s)
//
// Intended to be registered via container.Supply.
func NewServer() (Server, error) {
	config.SetDefault("http.addr", DefaultAddr)
	config.SetDefault("http.read_timeout", DefaultReadTimeout.String())
	config.SetDefault("http.write_timeout", DefaultWriteTimeout.String())
	config.SetDefault("http.idle_timeout", DefaultIdleTimeout.String())

	config.Describe("http.addr", "HTTP server listen address (host:port)")
	config.Describe("http.read_timeout", "Max duration for reading request headers and body")
	config.Describe("http.write_timeout", "Max duration for writing the response")
	config.Describe("http.idle_timeout", "Max duration for keep-alive connections to stay idle")

	addr := config.GetOr[string]("http.addr", DefaultAddr)
	readTimeout := config.GetOr[time.Duration]("http.read_timeout", DefaultReadTimeout)
	writeTimeout := config.GetOr[time.Duration]("http.write_timeout", DefaultWriteTimeout)
	idleTimeout := config.GetOr[time.Duration]("http.idle_timeout", DefaultIdleTimeout)

	mux := httpstd.NewServeMux()
	mux.HandleFunc("/", func(w httpstd.ResponseWriter, r *httpstd.Request) {
		w.WriteHeader(httpstd.StatusOK)
	})

	var handler httpstd.Handler = mux
	handler = middleware.Recover(handler)
	handler = middleware.RequestLogger(handler)
	handler = middleware.RequestID(handler)

	srv := &httpstd.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	container.AppendHook(container.Hook{
		Name: "http",
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

	return &serverImpl{
		mux:  mux,
		opts: &optionsRegistry{registered: make(map[string]struct{})},
	}, nil
}

type serverImpl struct {
	mux  *httpstd.ServeMux
	opts *optionsRegistry
}

func (s *serverImpl) Mux() *httpstd.ServeMux { return s.mux }

func (s *serverImpl) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		prefix: prefix,
		mux:    s.mux,
		opts:   s.opts,
	}
}
