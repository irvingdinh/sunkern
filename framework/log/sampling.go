package log

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// SamplingRate configures per-level log sampling. A rate of N means 1-in-N
// records at that level are passed through; the rest are silently dropped.
// Zero means no sampling (all records pass through).
//
// Only DEBUG and INFO support sampling — WARN and ERROR are always logged in
// full because they indicate conditions that need attention.
type SamplingRate struct {
	Debug int // 0 = log all, N = log 1-in-N
	Info  int // 0 = log all, N = log 1-in-N
}

// samplingHandler wraps an inner slog.Handler and drops log records based on
// per-level sampling rates. It uses lock-free atomic counters so the hot path
// (a sampled-out record) costs only an atomic increment and a modulo check.
//
// Counters are shared via pointer across WithAttrs/WithGroup-derived handlers
// so the sampling rate is global — all loggers that share a root sampling
// handler contribute to the same counter. This avoids violating the
// sync/atomic contract (atomic values must not be copied after first use).
type samplingHandler struct {
	inner    slog.Handler
	rates    [2]int64        // [0]=DEBUG, [1]=INFO
	counters *[2]atomic.Int64 // shared per-level monotonic counters
}

// newSamplingHandler wraps inner with per-level sampling. If all rates are
// zero (no sampling), it returns inner unchanged to avoid unnecessary
// indirection.
func newSamplingHandler(inner slog.Handler, rate SamplingRate) slog.Handler {
	if rate.Debug <= 0 && rate.Info <= 0 {
		return inner
	}
	h := &samplingHandler{
		inner:    inner,
		counters: new([2]atomic.Int64),
	}
	if rate.Debug > 0 {
		h.rates[0] = int64(rate.Debug)
	}
	if rate.Info > 0 {
		h.rates[1] = int64(rate.Info)
	}
	return h
}

func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *samplingHandler) Handle(ctx context.Context, r slog.Record) error {
	idx := -1
	switch {
	case r.Level < slog.LevelInfo:
		idx = 0 // DEBUG
	case r.Level < slog.LevelWarn:
		idx = 1 // INFO
	}
	// WARN and ERROR (idx == -1) always pass through.
	if idx >= 0 && h.rates[idx] > 0 {
		n := h.counters[idx].Add(1)
		if n%h.rates[idx] != 0 {
			return nil // sampled out
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &samplingHandler{
		inner:    h.inner.WithAttrs(attrs),
		rates:    h.rates,
		counters: h.counters,
	}
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	return &samplingHandler{
		inner:    h.inner.WithGroup(name),
		rates:    h.rates,
		counters: h.counters,
	}
}
