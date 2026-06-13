package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/domain/token"
)

// JWTManager is the JWT-based implementation of token.Manager and
// token.ManagerExtended.  It lives in the infrastructure layer so that
// the domain never depends on the JWT library.
type JWTManager struct {
	secret    string
	issuer    string
	audience  string
	accessTTL time.Duration
}

// NewJWTManager creates a new instance of JWTManager.
func NewJWTManager(secret, issuer, audience string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{secret: secret, issuer: issuer, audience: audience, accessTTL: accessTTL}
}

// accessTokenClaims defines the standard JWT claims plus the optional role.
type accessTokenClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role,omitempty"`
}

// Generate creates a JWT access token containing only the user ID.
// Satisfies token.Manager; kept for backward compatibility.
func (m *JWTManager) Generate(userID int64) (string, error) {
	return m.GenerateWithRole(userID, "")
}

// GenerateWithRole creates a JWT access token that embeds user ID and role.
// Satisfies token.ManagerExtended.
func (m *JWTManager) GenerateWithRole(userID int64, role string) (string, error) {
	now := time.Now().UTC()
	claims := accessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	if role != "" {
		claims.Role = role
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := t.SignedString([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

// Parse validates the JWT and returns the user ID.
// Satisfies token.Manager.
func (m *JWTManager) Parse(tokenString string) (int64, error) {
	userID, _, err := m.ParseWithRole(tokenString)
	return userID, err
}

// ParseWithRole validates the JWT and returns both user ID and role claim.
// Satisfies token.ManagerExtended.
func (m *JWTManager) ParseWithRole(tokenString string) (int64, string, error) {
	claims := &accessTokenClaims{}
	t, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(m.secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience))
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !t.Valid {
		return 0, "", fmt.Errorf("invalid token claims")
	}

	subStr := claims.Subject
	if subStr == "" {
		return 0, "", fmt.Errorf("sub claim is missing or not a string")
	}

	userID, err := strconv.ParseInt(subStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse sub claim as user ID: %w", err)
	}

	return userID, claims.Role, nil
}

// Compile-time interface compliance checks.
var _ token.Manager = (*JWTManager)(nil)
var _ token.ManagerExtended = (*JWTManager)(nil)
