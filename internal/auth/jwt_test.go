package auth

import (
	"testing"
)

func TestJWTManager(t *testing.T) {
	secret := "test-secret-key-12345"
	manager := NewJWTManager(secret)

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

	// Test invalid token parsing
	_, err = manager.Parse("completely.invalid.tokenstring")
	if err == nil {
		t.Error("expected error parsing invalid token, got nil")
	}
}
