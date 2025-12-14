package web

import (
	"testing"

	"github.com/seckatie/bookmarkd/internal/core/db"
)

// newTestDB creates a new in-memory SQLite database for testing.
func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return database
}

// newTestServer creates a new Server instance for testing.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	database := newTestDB(t)
	server, err := newServer(database)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	return server
}

// TestNewServer tests server initialization.
func TestNewServer(t *testing.T) {
	t.Run("creates server successfully", func(t *testing.T) {
		database := newTestDB(t)
		defer database.Close()

		server, err := newServer(database)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if server.db == nil {
			t.Error("expected db to be set")
		}
		if server.templates == nil {
			t.Error("expected templates to be loaded")
		}
		if server.staticFS == nil {
			t.Error("expected staticFS to be set")
		}
	})

	t.Run("loads all required templates", func(t *testing.T) {
		database := newTestDB(t)
		defer database.Close()

		server, _ := newServer(database)

		requiredTemplates := []string{
			"index.html",
			"bookmarks.html",
			"viewer.html",
			"archives.html",
			"archives_list.html",
			"archive_item.html",
			"bookmarklet.html",
			"bookmarklet_add.html",
			"nav.html",
		}

		for _, name := range requiredTemplates {
			if server.templates.Lookup(name) == nil {
				t.Errorf("expected template %q to be loaded", name)
			}
		}
	})
}
