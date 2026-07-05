package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/auth"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

const refreshCookieName = "amazonas_refresh"

func (s *Server) createGuestSession(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.auth.CreateGuest(r.Context())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No fue posible crear la sesion.", nil)
		return
	}
	s.setRefreshCookie(w, tokens.RefreshCookie, tokens.RefreshExpiresAt)
	s.writeJSON(w, http.StatusCreated, sessionResponse(tokens))
}

func (s *Server) refreshSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		s.writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "No hay una sesion renovable.", nil)
		return
	}
	tokens, err := s.auth.Refresh(r.Context(), cookie.Value)
	if err != nil {
		code := "UNAUTHORIZED"
		if errors.Is(err, auth.ErrInvalidRefresh) || errors.Is(err, repository.ErrSessionCompromised) {
			code = "REFRESH_REUSED"
		}
		s.clearRefreshCookie(w)
		s.writeError(w, r, http.StatusUnauthorized, code, "El refresh token no es valido.", nil)
		return
	}
	s.setRefreshCookie(w, tokens.RefreshCookie, tokens.RefreshExpiresAt)
	s.writeJSON(w, http.StatusOK, sessionResponse(tokens))
}

func (s *Server) currentSession(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r.Context())
	s.writeJSON(w, http.StatusOK, CurrentSessionResponse{
		GuestID: claims.Subject, SessionID: claims.SessionID, ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
	})
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r.Context())
	if err := s.auth.Revoke(r.Context(), claims.SessionID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No fue posible cerrar la sesion.", nil)
		return
	}
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func sessionResponse(tokens auth.SessionTokens) SessionResponse {
	return SessionResponse{
		GuestID: tokens.OwnerID, SessionID: tokens.SessionID, AccessToken: tokens.AccessToken, TokenType: "Bearer",
		AccessExpiresAt: tokens.AccessExpiresAt, RefreshExpiresAt: tokens.RefreshExpiresAt,
	}
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: value, Path: "/api/v1/sessions", HttpOnly: true,
		Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()),
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/api/v1/sessions", HttpOnly: true,
		Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
