package db

import (
	"os"
	"testing"
	"time"
)

// newTestDB creates a new in-memory SQLite database for testing.
// It runs migrations and returns the DB instance.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

// TestNewSQLiteDB tests database creation.
func TestNewSQLiteDB(t *testing.T) {
	t.Run("in-memory database", func(t *testing.T) {
		db, err := NewSQLiteDB(":memory:")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer db.Close()

		if db.db == nil {
			t.Error("expected db.db to be non-nil")
		}
		if db.eventListeners == nil {
			t.Error("expected eventListeners to be initialized")
		}
	})

	t.Run("file database", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "bookmarkd-test-*.db")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		db, err := NewSQLiteDB(tmpFile.Name())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer db.Close()

		if db.db == nil {
			t.Error("expected db.db to be non-nil")
		}
	})
}

// TestMigrate tests the migration system.
func TestMigrate(t *testing.T) {
	t.Run("applies migrations successfully", func(t *testing.T) {
		db, err := NewSQLiteDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}
		defer db.Close()

		if err := db.Migrate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify schema_migrations table exists and has entries
		var count int
		err = db.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		if err != nil {
			t.Fatalf("failed to query schema_migrations: %v", err)
		}
		if count == 0 {
			t.Error("expected migrations to be recorded")
		}
	})

	t.Run("migrations are idempotent", func(t *testing.T) {
		db, err := NewSQLiteDB(":memory:")
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}
		defer db.Close()

		// Run migrations twice
		if err := db.Migrate(); err != nil {
			t.Fatalf("first migration failed: %v", err)
		}
		if err := db.Migrate(); err != nil {
			t.Fatalf("second migration failed: %v", err)
		}

		// Verify bookmarks table exists by inserting a row
		_, err = db.db.Exec("INSERT INTO bookmarks (url, title, created_at) VALUES (?, ?, ?)",
			"https://example.com", "Test", time.Now().Format(time.RFC3339))
		if err != nil {
			t.Fatalf("failed to insert into bookmarks: %v", err)
		}
	})
}

// TestClose tests database close functionality.
func TestClose(t *testing.T) {
	db, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify database is closed by attempting a query
	_, err = db.db.Exec("SELECT 1")
	if err == nil {
		t.Error("expected error after close, got nil")
	}
}
