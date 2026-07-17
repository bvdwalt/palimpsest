package config_test

import (
	"testing"

	"github.com/bvdwalt/inkbase/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("AUTOSAVE_INTERVAL_SECONDS", "")

	cfg := config.Load()
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DBPath != "/data/inkbase.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/data/inkbase.db")
	}
	if cfg.AutosaveIntervalSeconds != 10 {
		t.Errorf("AutosaveIntervalSeconds = %d, want 10", cfg.AutosaveIntervalSeconds)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_PATH", "/tmp/custom.db")
	t.Setenv("AUTOSAVE_INTERVAL_SECONDS", "30")

	cfg := config.Load()
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.DBPath != "/tmp/custom.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/custom.db")
	}
	if cfg.AutosaveIntervalSeconds != 30 {
		t.Errorf("AutosaveIntervalSeconds = %d, want 30", cfg.AutosaveIntervalSeconds)
	}
}

func TestLoadInvalidAutosaveIntervalFallsBackToDefault(t *testing.T) {
	t.Setenv("AUTOSAVE_INTERVAL_SECONDS", "not-a-number")
	if cfg := config.Load(); cfg.AutosaveIntervalSeconds != 10 {
		t.Errorf("AutosaveIntervalSeconds = %d, want 10 (fallback on parse error)", cfg.AutosaveIntervalSeconds)
	}

	t.Setenv("AUTOSAVE_INTERVAL_SECONDS", "0")
	if cfg := config.Load(); cfg.AutosaveIntervalSeconds != 10 {
		t.Errorf("AutosaveIntervalSeconds = %d, want 10 (fallback on non-positive value)", cfg.AutosaveIntervalSeconds)
	}

	t.Setenv("AUTOSAVE_INTERVAL_SECONDS", "-5")
	if cfg := config.Load(); cfg.AutosaveIntervalSeconds != 10 {
		t.Errorf("AutosaveIntervalSeconds = %d, want 10 (fallback on negative value)", cfg.AutosaveIntervalSeconds)
	}
}
