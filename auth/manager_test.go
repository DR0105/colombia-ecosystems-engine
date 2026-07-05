package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

func testManager(t *testing.T) (*Manager, *repository.JSONRepository) {
	t.Helper()
	repo, err := repository.NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(repo, Config{
		Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "test-api", Audience: "test-web",
		AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager, repo
}

func TestGuestAccessAndRefreshRotation(t *testing.T) {
	manager, _ := testManager(t)
	tokens, err := manager.CreateGuest(context.Background())
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	claims, err := manager.ValidateAccess(context.Background(), tokens.AccessToken)
	if err != nil || claims.Subject != tokens.OwnerID || claims.SessionID != tokens.SessionID {
		t.Fatalf("claims = %+v, %v", claims, err)
	}
	rotated, err := manager.Refresh(context.Background(), tokens.RefreshCookie)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshCookie == tokens.RefreshCookie {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := manager.Refresh(context.Background(), tokens.RefreshCookie); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("refresh reuse error = %v", err)
	}
	if _, err := manager.ValidateAccess(context.Background(), rotated.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("access should be invalid after refresh reuse revocation: %v", err)
	}
}

func TestExpiredAndTamperedAccessTokens(t *testing.T) {
	manager, _ := testManager(t)
	now := time.Now().UTC()
	manager.now = func() time.Time { return now }
	tokens, err := manager.CreateGuest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(16 * time.Minute) }
	if _, err := manager.ValidateAccess(context.Background(), tokens.AccessToken); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expired token error = %v", err)
	}
	manager.now = func() time.Time { return now }
	tampered := tokens.AccessToken[:len(tokens.AccessToken)-1] + "x"
	if _, err := manager.ValidateAccess(context.Background(), tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token error = %v", err)
	}
}

func TestManagerRejectsShortSecret(t *testing.T) {
	repo, err := repository.NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewManager(repo, Config{Secret: []byte("short"), Issuer: "api", Audience: "web", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	if err == nil {
		t.Fatal("short JWT secret should fail")
	}
}
