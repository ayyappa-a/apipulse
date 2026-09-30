package middleware

import "context"

type contextKey string

const (
	contextKeyRequestID contextKey = "request_id"
)

// withRequestID stores a request ID in the context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}

// requestIDFromContext retrieves the request ID from the context.
// Returns an empty string if not present.
func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return v
	}
	return ""
}
