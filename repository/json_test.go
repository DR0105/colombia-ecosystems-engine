package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

const (
	testGameID    = "game_0123456789abcdef0123456789abcdef"
	testSessionID = "session_0123456789abcdef0123456789abcdef"
)

func TestJSONRepositoryGameLifecycleAndRestart(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewJSONRepository(dir)
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.CreateGame(context.Background(), StoredGame{
		ID: testGameID, OwnerID: "guest_owner", State: domain.GameState{ScenarioID: "amazonas_mvp", Round: 1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("version = %d, want 1", created.Version)
	}
	if _, err := repo.GetGame(context.Background(), testGameID, "other"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong owner error = %v", err)
	}
	created.State.Round = 2
	updated, err := repo.UpdateGame(context.Background(), testGameID, "guest_owner", 1, created.State)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 || updated.State.Round != 2 {
		t.Fatalf("unexpected update: %+v", updated)
	}
	if _, err := repo.UpdateGame(context.Background(), testGameID, "guest_owner", 1, created.State); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}

	restarted, err := NewJSONRepository(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := restarted.GetGame(context.Background(), testGameID, "guest_owner")
	if err != nil || loaded.Version != 2 || loaded.State.Round != 2 {
		t.Fatalf("restart load = %+v, %v", loaded, err)
	}
	page, err := restarted.ListGames(context.Background(), "guest_owner", 20, "")
	if err != nil || len(page.Games) != 1 {
		t.Fatalf("list = %+v, %v", page, err)
	}
}

func TestJSONRepositoryConcurrentVersionCheck(t *testing.T) {
	repo, err := NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateGame(context.Background(), StoredGame{ID: testGameID, OwnerID: "guest_owner", State: domain.GameState{Round: 0}})
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var conflicts atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func(round int) {
			defer wait.Done()
			_, err := repo.UpdateGame(context.Background(), testGameID, "guest_owner", 1, domain.GameState{Round: round})
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrVersionConflict):
				conflicts.Add(1)
			default:
				t.Errorf("unexpected update error: %v", err)
			}
		}(i + 1)
	}
	wait.Wait()
	if successes.Load() != 1 || conflicts.Load() != 7 {
		t.Fatalf("successes/conflicts = %d/%d", successes.Load(), conflicts.Load())
	}
}

func TestJSONRepositoryRefreshRotationDetectsReuse(t *testing.T) {
	repo, err := NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(time.Hour)
	_, err = repo.CreateSession(context.Background(), GuestSession{
		ID: testSessionID, OwnerID: "guest_owner", RefreshTokenHash: "old", ExpiresAt: expiry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RotateSession(context.Background(), testSessionID, "old", "new", expiry); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := repo.RotateSession(context.Background(), testSessionID, "old", "newer", expiry); !errors.Is(err, ErrSessionCompromised) {
		t.Fatalf("reuse error = %v", err)
	}
	session, err := repo.GetSession(context.Background(), testSessionID)
	if err != nil || session.RevokedAt == nil {
		t.Fatalf("session should be revoked: %+v, %v", session, err)
	}
}

func TestJSONRepositoryRejectsUnsafeIDs(t *testing.T) {
	repo, err := NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetGame(context.Background(), "../secret", "owner"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unsafe game ID error = %v", err)
	}
	if _, err := repo.GetSession(context.Background(), "session_../../secret"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unsafe session ID error = %v", err)
	}
}
