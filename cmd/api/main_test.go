package main

import (
	"strings"
	"testing"
)

func TestLoadConfigControlsTestPresets(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENABLE_TEST_PRESETS", "true")
	settings, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AppEnvironment != "development" || !settings.EnableTestPresets {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestLoadConfigRejectsTestPresetsInProduction(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENABLE_TEST_PRESETS", "true")
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "cannot be enabled in production") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigDisablesTestPresetsByDefault(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENABLE_TEST_PRESETS", "")
	settings, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if settings.EnableTestPresets {
		t.Fatal("test presets should be disabled by default")
	}
}
