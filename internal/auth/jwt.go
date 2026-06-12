package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/unowned-22/api/internal/domain/token"
)

// JWTManager is the JWT-based implementation of token.Manager and
// token.ManagerExtended.  It lives in the infrastructure layer so that
// the domain never depends on the JWT library.
type JWTManager struct {
	secret string
}

// NewJWTManager creates a new instance of JWTManager.
func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: secret}
}

// Generate creates a JWT access token containing only the user ID.
// Satisfies token.Manager; kept for backward compatibility.
func (m *JWTManager) Generate(userID int64) (string, error) {
	return m.GenerateWithRole(userID, "")
}

// GenerateWithRole creates a JWT access token that embeds user ID and role.
// Satisfies token.ManagerExtended.
func (m *JWTManager) GenerateWithRole(userID int64, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}
	if role != "" {
		claims["role"] = role
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
	t, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return 0, "", fmt.Errorf("invalid token claims")
	}

	subStr, ok := claims["sub"].(string)
	if !ok {
		return 0, "", fmt.Errorf("sub claim is missing or not a string")
	}

	userID, err := strconv.ParseInt(subStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse sub claim as user ID: %w", err)
	}

	role, _ := claims["role"].(string) // empty string if not present

	return userID, role, nil
}

// Compile-time interface compliance checks.
var _ token.Manager = (*JWTManager)(nil)
var _ token.ManagerExtended = (*JWTManager)(nil)
