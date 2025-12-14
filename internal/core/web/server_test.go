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
		if server.indexHTML == nil {
			t.Error("expected indexHTML to be loaded")
		}
		if server.bookmarksTmpl == nil {
			t.Error("expected bookmarksTmpl to be loaded")
		}
		if server.viewerTmpl == nil {
			t.Error("expected viewerTmpl to be loaded")
		}
		if server.archivesHTML == nil {
			t.Error("expected archivesHTML to be loaded")
		}
		if server.archivesListTmpl == nil {
			t.Error("expected archivesListTmpl to be loaded")
		}
		if server.archiveItemTmpl == nil {
			t.Error("expected archiveItemTmpl to be loaded")
		}
		if server.bookmarkletHTML == nil {
			t.Error("expected bookmarkletHTML to be loaded")
		}
		if server.bookmarkletAddTmpl == nil {
			t.Error("expected bookmarkletAddTmpl to be loaded")
		}
	})

	t.Run("loads templates with content", func(t *testing.T) {
		database := newTestDB(t)
		defer database.Close()

		server, _ := newServer(database)

		if len(server.indexHTML) == 0 {
			t.Error("expected indexHTML to have content")
		}
		if len(server.archivesHTML) == 0 {
			t.Error("expected archivesHTML to have content")
		}
		if len(server.bookmarkletHTML) == 0 {
			t.Error("expected bookmarkletHTML to have content")
		}
	})
}
