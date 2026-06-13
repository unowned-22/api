package auth

import "time"

// NewTokenManager creates a JWTManager using the provided configuration.
func NewTokenManager(secret, issuer, audience string, accessTTL time.Duration) *JWTManager {
	return NewJWTManager(secret, issuer, audience, accessTTL)
}
