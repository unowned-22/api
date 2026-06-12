package contextx

import "context"

// contextKey is an unexported type for all keys stored in a request context.
// Using a package-local type prevents key collisions with other packages that
// also store values in context.Context.
type contextKey int

const (
	userIDKey   contextKey = iota // int64 — authenticated user's primary key
	userRoleKey                   // string — role name embedded in the JWT claim
)

// SetUserID returns a new context carrying the authenticated user's ID.
func SetUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserID retrieves the authenticated user's ID from ctx.
// The second return value is false when no ID has been stored.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// SetUserRole returns a new context carrying the authenticated user's role name.
func SetUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, userRoleKey, role)
}

// UserRole retrieves the authenticated user's role from ctx.
// The second return value is false when no role has been stored.
func UserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}
