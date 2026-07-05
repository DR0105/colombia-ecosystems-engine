package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

var (
	gameIDPattern    = regexp.MustCompile(`^game_[a-f0-9]{32}$`)
	sessionIDPattern = regexp.MustCompile(`^session_[a-f0-9]{32}$`)
)

type JSONRepository struct {
	root         string
	gamesDir     string
	sessionsDir  string
	mu           sync.RWMutex
	gameLocks    sync.Map
	sessionLocks sync.Map
}

func NewJSONRepository(root string) (*JSONRepository, error) {
	if root == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	repo := &JSONRepository{
		root:        root,
		gamesDir:    filepath.Join(root, "games"),
		sessionsDir: filepath.Join(root, "sessions"),
	}
	for _, dir := range []string{repo.root, repo.gamesDir, repo.sessionsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create repository directory: %w", err)
		}
	}
	return repo, nil
}

func (r *JSONRepository) CreateGame(ctx context.Context, game StoredGame) (StoredGame, error) {
	if !gameIDPattern.MatchString(game.ID) {
		return StoredGame{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return StoredGame{}, err
	}
	lock := r.gameLock(game.ID)
	lock.Lock()
	defer lock.Unlock()
	path := r.gamePath(game.ID)
	if _, err := os.Stat(path); err == nil {
		return StoredGame{}, ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return StoredGame{}, err
	}
	if game.Version == 0 {
		game.Version = 1
	}
	now := time.Now().UTC()
	if game.CreatedAt.IsZero() {
		game.CreatedAt = now
	}
	game.UpdatedAt = now
	if err := r.writeJSON(path, game); err != nil {
		return StoredGame{}, err
	}
	return game, nil
}

func (r *JSONRepository) GetGame(ctx context.Context, id, ownerID string) (StoredGame, error) {
	if !gameIDPattern.MatchString(id) {
		return StoredGame{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return StoredGame{}, err
	}
	lock := r.gameLock(id)
	lock.RLock()
	defer lock.RUnlock()
	var game StoredGame
	if err := r.readJSON(r.gamePath(id), &game); err != nil {
		return StoredGame{}, err
	}
	if game.OwnerID != ownerID {
		return StoredGame{}, ErrForbidden
	}
	return game, nil
}

func (r *JSONRepository) ListGames(ctx context.Context, ownerID string, limit int, cursor string) (GamePage, error) {
	if err := ctx.Err(); err != nil {
		return GamePage{}, err
	}
	entries, err := os.ReadDir(r.gamesDir)
	if err != nil {
		return GamePage{}, fmt.Errorf("list games: %w", err)
	}
	games := make([]StoredGame, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var game StoredGame
		if err := r.readJSON(filepath.Join(r.gamesDir, entry.Name()), &game); err != nil {
			return GamePage{}, err
		}
		if game.OwnerID == ownerID {
			games = append(games, game)
		}
	}
	sort.Slice(games, func(i, j int) bool {
		if games[i].UpdatedAt.Equal(games[j].UpdatedAt) {
			return games[i].ID < games[j].ID
		}
		return games[i].UpdatedAt.After(games[j].UpdatedAt)
	})
	start := 0
	if cursor != "" {
		for i, game := range games {
			if game.ID == cursor {
				start = i + 1
				break
			}
		}
	}
	if limit <= 0 {
		limit = 20
	}
	end := start + limit
	if end > len(games) {
		end = len(games)
	}
	page := GamePage{Games: append([]StoredGame(nil), games[start:end]...)}
	if end < len(games) && end > 0 {
		page.NextCursor = games[end-1].ID
	}
	return page, nil
}

func (r *JSONRepository) CountGames(ctx context.Context, ownerID string) (int, error) {
	page, err := r.ListGames(ctx, ownerID, int(^uint(0)>>1), "")
	return len(page.Games), err
}

func (r *JSONRepository) UpdateGame(ctx context.Context, id, ownerID string, expectedVersion uint64, state domain.GameState) (StoredGame, error) {
	if !gameIDPattern.MatchString(id) {
		return StoredGame{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return StoredGame{}, err
	}
	lock := r.gameLock(id)
	lock.Lock()
	defer lock.Unlock()
	var game StoredGame
	if err := r.readJSON(r.gamePath(id), &game); err != nil {
		return StoredGame{}, err
	}
	if game.OwnerID != ownerID {
		return StoredGame{}, ErrForbidden
	}
	if game.Version != expectedVersion {
		return StoredGame{}, ErrVersionConflict
	}
	game.Version++
	game.UpdatedAt = time.Now().UTC()
	game.State = state
	if err := r.writeJSON(r.gamePath(id), game); err != nil {
		return StoredGame{}, err
	}
	return game, nil
}

func (r *JSONRepository) DeleteGame(ctx context.Context, id, ownerID string) error {
	if !gameIDPattern.MatchString(id) {
		return ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lock := r.gameLock(id)
	lock.Lock()
	defer lock.Unlock()
	var game StoredGame
	if err := r.readJSON(r.gamePath(id), &game); err != nil {
		return err
	}
	if game.OwnerID != ownerID {
		return ErrForbidden
	}
	if err := os.Remove(r.gamePath(id)); err != nil {
		return fmt.Errorf("delete game: %w", err)
	}
	return nil
}

func (r *JSONRepository) CreateSession(ctx context.Context, session GuestSession) (GuestSession, error) {
	if !sessionIDPattern.MatchString(session.ID) {
		return GuestSession{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return GuestSession{}, err
	}
	lock := r.sessionLock(session.ID)
	lock.Lock()
	defer lock.Unlock()
	path := r.sessionPath(session.ID)
	if _, err := os.Stat(path); err == nil {
		return GuestSession{}, ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return GuestSession{}, err
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if err := r.writeJSON(path, session); err != nil {
		return GuestSession{}, err
	}
	return session, nil
}

func (r *JSONRepository) GetSession(ctx context.Context, id string) (GuestSession, error) {
	if !sessionIDPattern.MatchString(id) {
		return GuestSession{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return GuestSession{}, err
	}
	lock := r.sessionLock(id)
	lock.RLock()
	defer lock.RUnlock()
	var session GuestSession
	if err := r.readJSON(r.sessionPath(id), &session); err != nil {
		return GuestSession{}, err
	}
	return session, nil
}

func (r *JSONRepository) RotateSession(ctx context.Context, id, expectedHash, newHash string, expiresAt time.Time) (GuestSession, error) {
	if !sessionIDPattern.MatchString(id) {
		return GuestSession{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return GuestSession{}, err
	}
	lock := r.sessionLock(id)
	lock.Lock()
	defer lock.Unlock()
	var session GuestSession
	if err := r.readJSON(r.sessionPath(id), &session); err != nil {
		return GuestSession{}, err
	}
	if session.RevokedAt != nil || time.Now().UTC().After(session.ExpiresAt) {
		return GuestSession{}, ErrForbidden
	}
	if session.RefreshTokenHash != expectedHash {
		now := time.Now().UTC()
		session.RevokedAt = &now
		_ = r.writeJSON(r.sessionPath(id), session)
		return GuestSession{}, ErrSessionCompromised
	}
	session.RefreshTokenHash = newHash
	session.ExpiresAt = expiresAt.UTC()
	if err := r.writeJSON(r.sessionPath(id), session); err != nil {
		return GuestSession{}, err
	}
	return session, nil
}

func (r *JSONRepository) RevokeSession(ctx context.Context, id string) error {
	if !sessionIDPattern.MatchString(id) {
		return ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lock := r.sessionLock(id)
	lock.Lock()
	defer lock.Unlock()
	var session GuestSession
	if err := r.readJSON(r.sessionPath(id), &session); err != nil {
		return err
	}
	now := time.Now().UTC()
	session.RevokedAt = &now
	return r.writeJSON(r.sessionPath(id), session)
}

func (r *JSONRepository) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	probe, err := os.CreateTemp(r.root, ".ready-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func (r *JSONRepository) gamePath(id string) string {
	return filepath.Join(r.gamesDir, id+".json")
}

func (r *JSONRepository) sessionPath(id string) string {
	return filepath.Join(r.sessionsDir, id+".json")
}

func (r *JSONRepository) gameLock(id string) *sync.RWMutex {
	lock, _ := r.gameLocks.LoadOrStore(id, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func (r *JSONRepository) sessionLock(id string) *sync.RWMutex {
	lock, _ := r.sessionLocks.LoadOrStore(id, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func (r *JSONRepository) readJSON(path string, target any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read repository file: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode repository file: %w", err)
	}
	return nil
}

func (r *JSONRepository) writeJSON(path string, value any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode repository file: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write repository file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit repository file: %w", err)
	}
	return nil
}
