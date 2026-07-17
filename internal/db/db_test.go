package db_test

import (
	"path/filepath"
	"testing"

	"github.com/bvdwalt/inkbase/internal/db"
)

func TestConnectPingFailureOnDirectoryPath(t *testing.T) {
	_, err := db.Connect(t.TempDir())
	if err == nil {
		t.Fatal("Connect with a directory path: err = nil, want an error")
	}
}

func TestConnectAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := db.Connect(path)
	if err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	first.Close()

	// Reconnecting to the same file should skip already-applied migrations without error.
	second, err := db.Connect(path)
	if err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	defer second.Close()

	var count int
	if err := second.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count == 0 {
		t.Error("schema_migrations is empty after two Connect calls, want recorded migrations")
	}
}
