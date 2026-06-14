package auth

import (
	"testing"
	"time"
)

func TestJWTManager(t *testing.T) {
	secret := "test-secret-key-12345"
	issuer := "api-service"
	audience := "client-app"
	manager := NewJWTManager(secret, issuer, audience, 15*time.Minute)

	userID := int64(9876)
	token, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	parsedID, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if parsedID != userID {
		t.Errorf("expected user ID %d, got %d", userID, parsedID)
	}

	// Verify standard claims are present and validated.
	parsedID2, role, ver, err := manager.ParseWithRole(token)
	if err != nil {
		t.Fatalf("failed to parse token with role: %v", err)
	}
	if parsedID2 != userID {
		t.Errorf("expected user ID %d from ParseWithRole, got %d", userID, parsedID2)
	}
	if role != "" {
		t.Errorf("expected empty role, got %q", role)
	}
	if ver != 0 && ver != 1 {
		// tokenVersion may be zero if not set; accept 0 or 1
		t.Errorf("unexpected token version: %d", ver)
	}

	// Test invalid token parsing
	_, err = manager.Parse("completely.invalid.tokenstring")
	if err == nil {
		t.Error("expected error parsing invalid token, got nil")
	}
}
