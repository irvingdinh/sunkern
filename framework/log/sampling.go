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
//
// When a request_id is present in the context, sampling is consistent: the
// request_id is hashed and the hash determines whether to keep or drop. All
// log entries for the same request get the same decision, preserving complete
// traces for sampled-in requests. When no request_id is present, a global
// atomic counter provides the 1-in-N decision per level.
type SamplingRate struct {
	Debug int // 0 = log all, N = log 1-in-N
	Info  int // 0 = log all, N = log 1-in-N
}

// samplingHandler wraps an inner slog.Handler and drops log records based on
// per-level sampling rates. It uses lock-free atomic counters so the hot path
// (a sampled-out record) costs only an atomic increment and a modulo check.
//
// When a request_id is available in the context, the handler uses consistent
// hashing (FNV-1a) instead of the counter, ensuring all logs for one request
// are either all kept or all dropped.
//
// Counters are shared via pointer across WithAttrs/WithGroup-derived handlers
// so the sampling rate is global — all loggers that share a root sampling
// handler contribute to the same counter. This avoids violating the
// sync/atomic contract (atomic values must not be copied after first use).
type samplingHandler struct {
	inner    slog.Handler
	rates    [2]int64         // [0]=DEBUG, [1]=INFO
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
		// Consistent sampling: if a request_id is in the context, hash it
		// so all log lines for the same request share the same keep/drop
		// decision. This preserves complete traces for sampled-in requests.
		if rid := RequestIDFromCtx(ctx); rid != "" {
			if fnv1a(rid)%uint32(h.rates[idx]) != 0 {
				return nil // sampled out (consistent per request)
			}
			return h.inner.Handle(ctx, r)
		}
		// No request context — fall back to global counter.
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

// fnv1a computes a 32-bit FNV-1a hash of s without heap allocation.
// Used for consistent per-request sampling decisions.
func fnv1a(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}
