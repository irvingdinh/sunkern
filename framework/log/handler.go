package log

import (
	"context"
	"errors"
	"log/slog"
)

// ---------------------------------------------------------------------------
// mergedHandler
// ---------------------------------------------------------------------------

// mergedHandler wraps two slog.Handler children (console + file) and delegates
// to both. Per-sink level filtering is applied in Handle via each child’s
// Enabled check.
type mergedHandler struct {
	console slog.Handler
	file    slog.Handler
}

func newMergedHandler(console, file slog.Handler) *mergedHandler {
	return &mergedHandler{console: console, file: file}
}

// Enabled always returns true so the slog.Logger does not short-circuit
// before Handle; each child still filters by level in Handle.
func (*mergedHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *mergedHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	if h.console.Enabled(ctx, r.Level) {
		if err := h.console.Handle(ctx, r); err != nil {
			errs = append(errs, err)
		}
	}
	if h.file.Enabled(ctx, r.Level) {
		if err := h.file.Handle(ctx, r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *mergedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &mergedHandler{
		console: h.console.WithAttrs(attrs),
		file:    h.file.WithAttrs(attrs),
	}
}

func (h *mergedHandler) WithGroup(name string) slog.Handler {
	return &mergedHandler{
		console: h.console.WithGroup(name),
		file:    h.file.WithGroup(name),
	}
}

// ---------------------------------------------------------------------------
// contextHandler
// ---------------------------------------------------------------------------

// contextHandler wraps an inner slog.Handler. On each Handle call it extracts
// request_id and user_id from the context and adds them as attributes before
// delegating.
type contextHandler struct {
	inner slog.Handler
}

func newContextHandler(inner slog.Handler) *contextHandler {
	return &contextHandler{inner: inner}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if rid := RequestIDFromCtx(ctx); rid != "" {
		r.AddAttrs(slog.String("request_id", rid))
	}
	if uid := UserIDFromCtx(ctx); uid != "" {
		r.AddAttrs(slog.String("user_id", uid))
	}
	return h.inner.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{inner: h.inner.WithGroup(name)}
}
