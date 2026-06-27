package contextx

import (
	"context"
	"testing"

	contextx2 "github.com/unowned-22/api/internal/contextx"
)

func TestUserID(t *testing.T) {
	ctx := context.Background()

	// Missing value returns zero + false.
	id, ok := contextx2.UserID(ctx)
	if ok || id != 0 {
		t.Fatalf("expected (0, false) on empty context, got (%d, %v)", id, ok)
	}

	// Stored value is returned correctly.
	ctx = contextx2.SetUserID(ctx, 42)
	id, ok = contextx2.UserID(ctx)
	if !ok || id != 42 {
		t.Fatalf("expected (42, true), got (%d, %v)", id, ok)
	}
}

func TestUserRole(t *testing.T) {
	ctx := context.Background()

	// Missing value returns empty string + false.
	role, ok := contextx2.UserRole(ctx)
	if ok || role != "" {
		t.Fatalf("expected (\"\", false) on empty context, got (%q, %v)", role, ok)
	}

	// Stored value is returned correctly.
	ctx = contextx2.SetUserRole(ctx, "admin")
	role, ok = contextx2.UserRole(ctx)
	if !ok || role != "admin" {
		t.Fatalf("expected (\"admin\", true), got (%q, %v)", role, ok)
	}
}

func TestIndependentKeys(t *testing.T) {
	ctx := contextx2.SetUserID(context.Background(), 7)
	ctx = contextx2.SetUserRole(ctx, "moderator")

	id, ok := contextx2.UserID(ctx)
	if !ok || id != 7 {
		t.Errorf("expected UserID=7, got (%d, %v)", id, ok)
	}

	role, ok := contextx2.UserRole(ctx)
	if !ok || role != "moderator" {
		t.Errorf("expected UserRole=moderator, got (%q, %v)", role, ok)
	}
}

func TestOverwrite(t *testing.T) {
	ctx := contextx2.SetUserID(context.Background(), 1)
	ctx = contextx2.SetUserID(ctx, 99)

	id, ok := contextx2.UserID(ctx)
	if !ok || id != 99 {
		t.Errorf("expected overwritten UserID=99, got (%d, %v)", id, ok)
	}
}
