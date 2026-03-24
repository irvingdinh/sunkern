package log

import "context"

// Context keys for request_id and user_id live in this package so
// [WithRequestID], [RequestIDFromCtx], and related helpers stay next to the
// log pipeline ([contextHandler]). If multiple framework packages need the
// same keys without importing log, introduce a small dedicated package (e.g.
// framework/ctx) and depend on it from here and from HTTP middleware.

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyUserID
)

// WithRequestID returns a copy of ctx with the request ID stored.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestIDFromCtx extracts the request ID from ctx, or returns "".
func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// WithUserID returns a copy of ctx with the user ID stored.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// UserIDFromCtx extracts the user ID from ctx, or returns "".
func UserIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok {
		return v
	}
	return ""
}
