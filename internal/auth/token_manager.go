package auth

// NewTokenManager Factory or helper functions for TokenManager if needed.
// Concrete JWTManager resides in jwt.go.
func NewTokenManager(secret string) *JWTManager {
	return NewJWTManager(secret)
}
