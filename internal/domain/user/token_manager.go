package user

// TokenManager is the primary auth contract used by services and middleware.
// It handles access token generation and parsing (user ID only).
// The public interface must remain stable.
type TokenManager interface {
	Generate(userID int64) (string, error)
	Parse(token string) (int64, error)
}

// TokenManagerExtended is an optional extension of TokenManager used when
// role information must be embedded in or extracted from the token.
// JWTManager implements both interfaces.
// Services that need role-aware tokens type-assert to this interface.
type TokenManagerExtended interface {
	TokenManager
	GenerateWithRole(userID int64, role string) (string, error)
	ParseWithRole(token string) (int64, string, error)
}
