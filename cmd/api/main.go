package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	gameapi "github.com/josephsae/colombia-ecosystems-engine/api"
	"github.com/josephsae/colombia-ecosystems-engine/auth"
	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

type config struct {
	HTTPAddr          string
	DataDir           string
	AppEnvironment    string
	JWTSecret         string
	JWTIssuer         string
	JWTAudience       string
	AllowedOrigins    []string
	CookieSecure      bool
	EnableTestPresets bool
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	ShutdownTTL       time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	settings, err := loadConfig()
	if err != nil {
		logger.Error("invalid_configuration", "error", err)
		os.Exit(1)
	}
	catalog, err := content.LoadEmbedded()
	if err != nil {
		logger.Error("load_catalog", "error", err)
		os.Exit(1)
	}
	repo, err := repository.NewJSONRepository(settings.DataDir)
	if err != nil {
		logger.Error("create_repository", "error", err)
		os.Exit(1)
	}
	authManager, err := auth.NewManager(repo, auth.Config{
		Secret: []byte(settings.JWTSecret), Issuer: settings.JWTIssuer, Audience: settings.JWTAudience,
		AccessTTL: settings.AccessTTL, RefreshTTL: settings.RefreshTTL,
	})
	if err != nil {
		logger.Error("create_auth_manager", "error", err)
		os.Exit(1)
	}
	apiServer, err := gameapi.NewServer(repo, authManager, catalog, gameapi.Config{
		AllowedOrigins: settings.AllowedOrigins, CookieSecure: settings.CookieSecure,
		EnableTestPresets: settings.EnableTestPresets, RefreshTTL: settings.RefreshTTL, MaxGames: 20, Logger: logger,
	}, gameapi.OpenAPISpec())
	if err != nil {
		logger.Error("create_api", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: settings.HTTPAddr, Handler: apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() {
		logger.Info("api_started", "address", settings.HTTPAddr, "docs", "/docs/", "environment", settings.AppEnvironment, "test_presets_enabled", settings.EnableTestPresets)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api_stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), settings.ShutdownTTL)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api_shutdown", "error", err)
		os.Exit(1)
	}
	logger.Info("api_stopped")
}

func loadConfig() (config, error) {
	result := config{
		HTTPAddr: env("HTTP_ADDR", ":8080"), DataDir: env("DATA_DIR", "./data"),
		AppEnvironment: env("APP_ENV", "development"),
		JWTSecret:      os.Getenv("JWT_SECRET"), JWTIssuer: env("JWT_ISSUER", "colombia-ecosystems-api"),
		JWTAudience:    env("JWT_AUDIENCE", "colombia-ecosystems-web"),
		AllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
	}
	if len(result.JWTSecret) < 32 {
		return config{}, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	if result.AppEnvironment != "development" && result.AppEnvironment != "test" && result.AppEnvironment != "production" {
		return config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	var err error
	if result.CookieSecure, err = strconv.ParseBool(env("COOKIE_SECURE", "false")); err != nil {
		return config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	if result.EnableTestPresets, err = strconv.ParseBool(env("ENABLE_TEST_PRESETS", "false")); err != nil {
		return config{}, fmt.Errorf("ENABLE_TEST_PRESETS: %w", err)
	}
	if result.AppEnvironment == "production" && result.EnableTestPresets {
		return config{}, fmt.Errorf("test presets cannot be enabled in production")
	}
	if result.AccessTTL, err = time.ParseDuration(env("ACCESS_TOKEN_TTL", "15m")); err != nil {
		return config{}, fmt.Errorf("ACCESS_TOKEN_TTL: %w", err)
	}
	if result.RefreshTTL, err = time.ParseDuration(env("REFRESH_TOKEN_TTL", "720h")); err != nil {
		return config{}, fmt.Errorf("REFRESH_TOKEN_TTL: %w", err)
	}
	if result.ShutdownTTL, err = time.ParseDuration(env("SHUTDOWN_TIMEOUT", "10s")); err != nil {
		return config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
	}
	return result, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
