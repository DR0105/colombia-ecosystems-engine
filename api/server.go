package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/auth"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

const maxBodyBytes = 1 << 20

var gameIDPattern = regexp.MustCompile(`^game_[a-f0-9]{32}$`)

type Config struct {
	AllowedOrigins []string
	CookieSecure   bool
	RefreshTTL     time.Duration
	MaxGames       int
	Logger         *slog.Logger
}

type Server struct {
	repository repository.Repository
	auth       *auth.Manager
	catalog    domain.Catalog
	config     Config
	mux        *http.ServeMux
	origins    map[string]bool
	openAPI    []byte
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	claimsKey    contextKey = "claims"
)

func NewServer(repo repository.Repository, authManager *auth.Manager, catalog domain.Catalog, config Config, openAPI []byte) (*Server, error) {
	if repo == nil || authManager == nil {
		return nil, fmt.Errorf("repository and auth manager are required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.MaxGames <= 0 {
		config.MaxGames = 20
	}
	if config.RefreshTTL <= 0 {
		config.RefreshTTL = 30 * 24 * time.Hour
	}
	server := &Server{
		repository: repo, auth: authManager, catalog: catalog, config: config,
		mux: http.NewServeMux(), origins: make(map[string]bool), openAPI: openAPI,
	}
	for _, origin := range config.AllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			server.origins[origin] = true
		}
	}
	server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.requestIDMiddleware(s.recoverMiddleware(s.loggingMiddleware(s.securityMiddleware(s.corsMiddleware(s.mux)))))
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/v1/sessions/guest", s.createGuestSession)
	s.mux.HandleFunc("POST /api/v1/sessions/refresh", s.requireAllowedOrigin(s.refreshSession))
	s.mux.Handle("GET /api/v1/sessions/current", s.requireAuth(http.HandlerFunc(s.currentSession)))
	s.mux.Handle("DELETE /api/v1/sessions/current", s.requireAuth(s.requireAllowedOriginHandler(http.HandlerFunc(s.deleteSession))))

	s.mux.Handle("POST /api/v1/games", s.requireAuth(http.HandlerFunc(s.createGame)))
	s.mux.Handle("GET /api/v1/games", s.requireAuth(http.HandlerFunc(s.listGames)))
	s.mux.Handle("GET /api/v1/games/{gameId}", s.requireAuth(http.HandlerFunc(s.getGame)))
	s.mux.Handle("POST /api/v1/games/{gameId}/commands", s.requireAuth(http.HandlerFunc(s.applyGameCommand)))
	s.mux.Handle("DELETE /api/v1/games/{gameId}", s.requireAuth(http.HandlerFunc(s.deleteGame)))

	s.mux.HandleFunc("GET /api/v1/catalog", s.getCatalog)
	s.mux.HandleFunc("GET /api/v1/catalog/cards/{cardId}", s.getCard)
	s.mux.HandleFunc("GET /health/live", s.live)
	s.mux.HandleFunc("GET /health/ready", s.ready)
	s.mux.HandleFunc("GET /openapi.yaml", s.openAPISpec)
	s.mux.HandleFunc("GET /docs/", s.swaggerUI)
	s.mux.HandleFunc("/", s.notFound)
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			s.writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Se requiere un token Bearer.", nil)
			return
		}
		claims, err := s.auth.ValidateAccess(r.Context(), parts[1])
		if err != nil {
			code := "UNAUTHORIZED"
			if err == auth.ErrExpiredToken {
				code = "TOKEN_EXPIRED"
			}
			s.writeError(w, r, http.StatusUnauthorized, code, "El token no es valido.", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !s.origins[origin] {
				s.writeError(w, r, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "El origen no esta permitido.", nil)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAllowedOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !s.origins[origin] {
			s.writeError(w, r, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "El origen no esta permitido.", nil)
			return
		}
		next(w, r)
	}
}

func (s *Server) requireAllowedOriginHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requireAllowedOrigin(next.ServeHTTP)(w, r)
	})
}

func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newID("req")
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		s.config.Logger.Info("http_request", "method", r.Method, "path", r.URL.Path, "status", wrapped.status,
			"duration_ms", time.Since(start).Milliseconds(), "request_id", requestID(r.Context()))
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				s.config.Logger.Error("http_panic", "error", value, "request_id", requestID(r.Context()))
				s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ocurrio un error interno.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); mediaType != "application/json" {
		s.writeError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type debe ser application/json.", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			s.writeError(w, r, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "El cuerpo supera 1 MiB.", nil)
		} else {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "El cuerpo JSON no es valido.", map[string]any{"cause": err.Error()})
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "Solo se permite un objeto JSON.", nil)
		return false
	}
	return true
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	s.writeJSON(w, status, ErrorResponse{Error: ErrorBody{Code: code, Message: message, RequestID: requestID(r.Context()), Details: details}})
}

func claimsFrom(ctx context.Context) auth.Claims {
	claims, _ := ctx.Value(claimsKey).(auth.Claims)
	return claims
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func newID(prefix string) string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(data)
}
