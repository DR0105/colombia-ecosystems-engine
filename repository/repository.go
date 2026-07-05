package repository

import (
	"context"
	"errors"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrForbidden          = errors.New("forbidden")
	ErrAlreadyExists      = errors.New("already exists")
	ErrVersionConflict    = errors.New("version conflict")
	ErrSessionCompromised = errors.New("session refresh token was already rotated")
)

type StoredGame struct {
	ID        string           `json:"id"`
	OwnerID   string           `json:"ownerId"`
	Version   uint64           `json:"version"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	State     domain.GameState `json:"state"`
}

type GuestSession struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"ownerId"`
	RefreshTokenHash string     `json:"refreshTokenHash"`
	CreatedAt        time.Time  `json:"createdAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
}

type GamePage struct {
	Games      []StoredGame
	NextCursor string
}

type Repository interface {
	CreateGame(context.Context, StoredGame) (StoredGame, error)
	GetGame(context.Context, string, string) (StoredGame, error)
	ListGames(context.Context, string, int, string) (GamePage, error)
	CountGames(context.Context, string) (int, error)
	UpdateGame(context.Context, string, string, uint64, domain.GameState) (StoredGame, error)
	DeleteGame(context.Context, string, string) error

	CreateSession(context.Context, GuestSession) (GuestSession, error)
	GetSession(context.Context, string) (GuestSession, error)
	RotateSession(context.Context, string, string, string, time.Time) (GuestSession, error)
	RevokeSession(context.Context, string) error
	Ready(context.Context) error
}
