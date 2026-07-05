package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

var refreshSessionPattern = regexp.MustCompile(`^session_[a-f0-9]{32}$`)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("token expired")
	ErrInvalidRefresh = errors.New("invalid refresh token")
)

type Config struct {
	Secret     []byte
	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type Claims struct {
	Subject   string `json:"sub"`
	SessionID string `json:"sid"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	TokenID   string `json:"jti"`
}

type SessionTokens struct {
	OwnerID          string
	SessionID        string
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshCookie    string
	RefreshExpiresAt time.Time
}

type Manager struct {
	repository repository.Repository
	config     Config
	now        func() time.Time
}

func NewManager(repo repository.Repository, config Config) (*Manager, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if len(config.Secret) < 32 {
		return nil, fmt.Errorf("JWT secret must contain at least 32 bytes")
	}
	if config.Issuer == "" || config.Audience == "" {
		return nil, fmt.Errorf("JWT issuer and audience are required")
	}
	if config.AccessTTL <= 0 || config.RefreshTTL <= 0 {
		return nil, fmt.Errorf("token TTL values must be positive")
	}
	return &Manager{repository: repo, config: config, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (m *Manager) CreateGuest(ctx context.Context) (SessionTokens, error) {
	ownerID, err := randomID("guest", 16)
	if err != nil {
		return SessionTokens{}, err
	}
	sessionID, err := randomID("session", 16)
	if err != nil {
		return SessionTokens{}, err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return SessionTokens{}, err
	}
	now := m.now()
	refreshExpiry := now.Add(m.config.RefreshTTL)
	_, err = m.repository.CreateSession(ctx, repository.GuestSession{
		ID:               sessionID,
		OwnerID:          ownerID,
		RefreshTokenHash: hashRefresh(refresh),
		CreatedAt:        now,
		ExpiresAt:        refreshExpiry,
	})
	if err != nil {
		return SessionTokens{}, err
	}
	return m.issue(ownerID, sessionID, refresh, refreshExpiry)
}

func (m *Manager) Refresh(ctx context.Context, cookieValue string) (SessionTokens, error) {
	sessionID, refresh, err := parseRefreshCookie(cookieValue)
	if err != nil {
		return SessionTokens{}, err
	}
	newRefresh, err := randomToken(32)
	if err != nil {
		return SessionTokens{}, err
	}
	expiry := m.now().Add(m.config.RefreshTTL)
	session, err := m.repository.RotateSession(ctx, sessionID, hashRefresh(refresh), hashRefresh(newRefresh), expiry)
	if errors.Is(err, repository.ErrSessionCompromised) {
		return SessionTokens{}, ErrInvalidRefresh
	}
	if err != nil {
		return SessionTokens{}, err
	}
	return m.issue(session.OwnerID, session.ID, newRefresh, expiry)
}

func (m *Manager) ValidateAccess(ctx context.Context, token string) (Claims, error) {
	claims, err := m.parseJWT(token)
	if err != nil {
		return Claims{}, err
	}
	session, err := m.repository.GetSession(ctx, claims.SessionID)
	if err != nil || session.RevokedAt != nil || m.now().After(session.ExpiresAt) || session.OwnerID != claims.Subject {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) Revoke(ctx context.Context, sessionID string) error {
	return m.repository.RevokeSession(ctx, sessionID)
}

func (m *Manager) issue(ownerID, sessionID, refresh string, refreshExpiry time.Time) (SessionTokens, error) {
	now := m.now()
	accessExpiry := now.Add(m.config.AccessTTL)
	jti, err := randomID("token", 12)
	if err != nil {
		return SessionTokens{}, err
	}
	access, err := m.signJWT(Claims{
		Subject: ownerID, SessionID: sessionID, Issuer: m.config.Issuer, Audience: m.config.Audience,
		IssuedAt: now.Unix(), ExpiresAt: accessExpiry.Unix(), TokenID: jti,
	})
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		OwnerID: ownerID, SessionID: sessionID, AccessToken: access, AccessExpiresAt: accessExpiry,
		RefreshCookie: sessionID + "." + refresh, RefreshExpiresAt: refreshExpiry,
	}, nil
}

func (m *Manager) signJWT(claims Claims) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := encodeSegment(header) + "." + encodeSegment(payload)
	signature := m.signature(unsigned)
	return unsigned + "." + encodeSegment(signature), nil
}

func (m *Manager) parseJWT(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var header map[string]string
	if json.Unmarshal(headerData, &header) != nil || header["alg"] != "HS256" || header["typ"] != "JWT" {
		return Claims{}, ErrInvalidToken
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(provided, m.signature(parts[0]+"."+parts[1])) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.Issuer != m.config.Issuer || claims.Audience != m.config.Audience || claims.Subject == "" || claims.SessionID == "" {
		return Claims{}, ErrInvalidToken
	}
	now := m.now().Unix()
	if claims.ExpiresAt <= now {
		return Claims{}, ErrExpiredToken
	}
	if claims.IssuedAt > now+60 {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) signature(value string) []byte {
	mac := hmac.New(sha256.New, m.config.Secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func randomID(prefix string, size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate identifier: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(data), nil
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hashRefresh(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func parseRefreshCookie(value string) (string, string, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 || !refreshSessionPattern.MatchString(parts[0]) || parts[1] == "" {
		return "", "", ErrInvalidRefresh
	}
	return parts[0], parts[1], nil
}

func encodeSegment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
