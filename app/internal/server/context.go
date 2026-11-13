package server

import "context"

type ctxKey int

const requestIDKey ctxKey = iota

// RequestIDFrom returns the request ID attached by the request-ID middleware.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
