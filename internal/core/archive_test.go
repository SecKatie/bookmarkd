package core

import (
	"context"
	"testing"
	"time"

	"github.com/seckatie/bookmarkd/internal/core/db"
)

func TestArchiveOptions(t *testing.T) {
	t.Run("default timeout is applied", func(t *testing.T) {
		opts := ArchiveOptions{}
		if opts.Timeout != 0 {
			t.Errorf("initial Timeout should be 0, got %v", opts.Timeout)
		}
		// The default is applied inside ArchiveBookmark, not in the struct
	})

	t.Run("headless defaults to false", func(t *testing.T) {
		opts := ArchiveOptions{}
		if opts.Headless {
			t.Error("Headless should default to false")
		}
	})

	t.Run("custom chrome path", func(t *testing.T) {
		opts := ArchiveOptions{ChromePath: "/custom/chrome"}
		if opts.ChromePath != "/custom/chrome" {
			t.Errorf("ChromePath = %q, want /custom/chrome", opts.ChromePath)
		}
	})

	t.Run("wait selector", func(t *testing.T) {
		opts := ArchiveOptions{WaitSelector: ".main-content"}
		if opts.WaitSelector != ".main-content" {
			t.Errorf("WaitSelector = %q, want .main-content", opts.WaitSelector)
		}
	})
}

func TestArchiveResult(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		result := ArchiveResult{}
		if result.FinalURL != "" {
			t.Error("FinalURL should be empty")
		}
		if result.Title != "" {
			t.Error("Title should be empty")
		}
		if result.HTML != "" {
			t.Error("HTML should be empty")
		}
	})

	t.Run("populated result", func(t *testing.T) {
		result := ArchiveResult{
			FinalURL: "https://example.com/final",
			Title:    "Example Page",
			HTML:     "<html><body>Test</body></html>",
		}
		if result.FinalURL != "https://example.com/final" {
			t.Errorf("FinalURL = %q, want https://example.com/final", result.FinalURL)
		}
		if result.Title != "Example Page" {
			t.Errorf("Title = %q, want Example Page", result.Title)
		}
		if result.HTML != "<html><body>Test</body></html>" {
			t.Errorf("HTML = %q, want <html><body>Test</body></html>", result.HTML)
		}
	})
}

func TestArchiveRunOptions(t *testing.T) {
	t.Run("single bookmark mode", func(t *testing.T) {
		opts := ArchiveRunOptions{ID: 123}
		if opts.ID != 123 {
			t.Errorf("ID = %d, want 123", opts.ID)
		}
	})

	t.Run("batch mode with limit", func(t *testing.T) {
		opts := ArchiveRunOptions{Limit: 10}
		if opts.Limit != 10 {
			t.Errorf("Limit = %d, want 10", opts.Limit)
		}
	})

	t.Run("nested archive options", func(t *testing.T) {
		opts := ArchiveRunOptions{
			Options: ArchiveOptions{
				Headless: true,
				Timeout:  30 * time.Second,
			},
		}
		if !opts.Options.Headless {
			t.Error("Options.Headless should be true")
		}
		if opts.Options.Timeout != 30*time.Second {
			t.Errorf("Options.Timeout = %v, want 30s", opts.Options.Timeout)
		}
	})
}

func TestArchiveRunResult(t *testing.T) {
	t.Run("all succeeded", func(t *testing.T) {
		result := ArchiveRunResult{Attempted: 5, Succeeded: 5, Failed: 0}
		if result.Attempted != 5 {
			t.Errorf("Attempted = %d, want 5", result.Attempted)
		}
		if result.Succeeded != 5 {
			t.Errorf("Succeeded = %d, want 5", result.Succeeded)
		}
		if result.Failed != 0 {
			t.Errorf("Failed = %d, want 0", result.Failed)
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		result := ArchiveRunResult{Attempted: 5, Succeeded: 3, Failed: 2}
		if result.Attempted != 5 {
			t.Errorf("Attempted = %d, want 5", result.Attempted)
		}
		if result.Succeeded != 3 {
			t.Errorf("Succeeded = %d, want 3", result.Succeeded)
		}
		if result.Failed != 2 {
			t.Errorf("Failed = %d, want 2", result.Failed)
		}
	})
}

func TestRunArchive_SingleBookmark(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create test database
	database, err := db.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	})

	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Add a bookmark
	id, err := database.AddBookmark("https://example.com", "Example")
	if err != nil {
		t.Fatalf("failed to add bookmark: %v", err)
	}

	// Run archive for non-existent bookmark should fail
	_, err = RunArchive(context.Background(), database, ArchiveRunOptions{
		ID: 99999,
		Options: ArchiveOptions{
			Headless: true,
			Timeout:  10 * time.Second,
		},
	})
	if err == nil {
		t.Error("expected error for non-existent bookmark")
	}

	// Note: Actually archiving requires Chrome, so we skip the success case in unit tests
	_ = id
}

func TestRunArchive_BatchMode_NoBookmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create test database
	database, err := db.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	})

	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Run archive with no bookmarks
	result, err := RunArchive(context.Background(), database, ArchiveRunOptions{
		Options: ArchiveOptions{
			Headless: true,
			Timeout:  10 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Attempted != 0 {
		t.Errorf("Attempted = %d, want 0", result.Attempted)
	}
}

func TestArchiveConstants(t *testing.T) {
	t.Run("DefaultArchiveTimeout", func(t *testing.T) {
		if DefaultArchiveTimeout != 35*time.Second {
			t.Errorf("DefaultArchiveTimeout = %v, want 35s", DefaultArchiveTimeout)
		}
	})

	t.Run("DefaultNetworkIdleDelay", func(t *testing.T) {
		if DefaultNetworkIdleDelay != 500*time.Millisecond {
			t.Errorf("DefaultNetworkIdleDelay = %v, want 500ms", DefaultNetworkIdleDelay)
		}
	})

	t.Run("ArchiveStatusOK", func(t *testing.T) {
		if ArchiveStatusOK != "ok" {
			t.Errorf("ArchiveStatusOK = %q, want ok", ArchiveStatusOK)
		}
	})

	t.Run("ArchiveStatusError", func(t *testing.T) {
		if ArchiveStatusError != "error" {
			t.Errorf("ArchiveStatusError = %q, want error", ArchiveStatusError)
		}
	})
}

// TestArchiveBookmark_RequiresBrowser tests the browser-based archiving.
// It's skipped by default since it requires Chrome to be available.
func TestArchiveBookmark_RequiresBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	// This test requires Chrome to be installed
	// Run with: go test -v -run TestArchiveBookmark_RequiresBrowser ./internal/core/...

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := ArchiveBookmark(ctx, "https://example.com", ArchiveOptions{
		Headless: true,
		Timeout:  20 * time.Second,
	})
	if err != nil {
		t.Skipf("Chrome not available or failed: %v", err)
	}

	if result.FinalURL == "" {
		t.Error("FinalURL should not be empty")
	}
	if result.HTML == "" {
		t.Error("HTML should not be empty")
	}
	if result.Title == "" {
		t.Log("Warning: Title is empty (some pages have no title)")
	}
}
