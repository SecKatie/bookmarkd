package db

import (
	"errors"
	"strings"
	"testing"
)

// TestAddBookmark tests bookmark creation.
func TestAddBookmark(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("creates bookmark successfully", func(t *testing.T) {
		id, err := db.AddBookmark("https://example.com", "Example Site")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("assigns sequential IDs", func(t *testing.T) {
		id1, _ := db.AddBookmark("https://site1.com", "Site 1")
		id2, _ := db.AddBookmark("https://site2.com", "Site 2")

		if id2 <= id1 {
			t.Errorf("expected id2 (%d) > id1 (%d)", id2, id1)
		}
	})
}

// TestGetBookmark tests retrieving a single bookmark.
func TestGetBookmark(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("retrieves existing bookmark", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "Example Site")

		b, err := db.GetBookmark(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if b.ID != id {
			t.Errorf("expected ID %d, got %d", id, b.ID)
		}
		if b.URL != "https://example.com" {
			t.Errorf("expected URL 'https://example.com', got %q", b.URL)
		}
		if b.Title != "Example Site" {
			t.Errorf("expected Title 'Example Site', got %q", b.Title)
		}
		if b.CreatedAt == "" {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		_, err := db.GetBookmark(99999)
		if err == nil {
			t.Error("expected error for non-existent bookmark, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestListBookmarks tests listing bookmarks.
func TestListBookmarks(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("returns empty list when no bookmarks", func(t *testing.T) {
		bookmarks, err := db.ListBookmarks(0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 0 {
			t.Errorf("expected empty list, got %d items", len(bookmarks))
		}
	})

	t.Run("returns all bookmarks", func(t *testing.T) {
		db.AddBookmark("https://site1.com", "Site 1")
		db.AddBookmark("https://site2.com", "Site 2")
		db.AddBookmark("https://site3.com", "Site 3")

		bookmarks, err := db.ListBookmarks(0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 3 {
			t.Errorf("expected 3 bookmarks, got %d", len(bookmarks))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		// Database already has 3 bookmarks from previous subtest
		bookmarks, err := db.ListBookmarks(2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 2 {
			t.Errorf("expected 2 bookmarks with limit, got %d", len(bookmarks))
		}
	})

	t.Run("orders by created_at DESC", func(t *testing.T) {
		// Create fresh database for this test
		db2 := newTestDB(t)
		defer db2.Close()

		// Insert bookmarks with explicit different timestamps to ensure ordering
		_, err := db2.db.Exec("INSERT INTO bookmarks (url, title, created_at) VALUES (?, ?, ?)",
			"https://first.com", "First", "2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("failed to insert first bookmark: %v", err)
		}
		_, err = db2.db.Exec("INSERT INTO bookmarks (url, title, created_at) VALUES (?, ?, ?)",
			"https://second.com", "Second", "2024-01-02T00:00:00Z")
		if err != nil {
			t.Fatalf("failed to insert second bookmark: %v", err)
		}
		_, err = db2.db.Exec("INSERT INTO bookmarks (url, title, created_at) VALUES (?, ?, ?)",
			"https://third.com", "Third", "2024-01-03T00:00:00Z")
		if err != nil {
			t.Fatalf("failed to insert third bookmark: %v", err)
		}

		bookmarks, _ := db2.ListBookmarks(0)

		// Most recent (latest created_at) should be first
		if bookmarks[0].Title != "Third" {
			t.Errorf("expected most recent bookmark first, got %q", bookmarks[0].Title)
		}
		if bookmarks[1].Title != "Second" {
			t.Errorf("expected second most recent bookmark second, got %q", bookmarks[1].Title)
		}
		if bookmarks[2].Title != "First" {
			t.Errorf("expected oldest bookmark last, got %q", bookmarks[2].Title)
		}
	})
}

// TestUpdateBookmark tests updating a bookmark.
func TestUpdateBookmark(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("updates existing bookmark", func(t *testing.T) {
		id, _ := db.AddBookmark("https://old.com", "Old Title")

		err := db.UpdateBookmark(id, "https://new.com", "New Title")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		b, _ := db.GetBookmark(id)
		if b.URL != "https://new.com" {
			t.Errorf("expected URL 'https://new.com', got %q", b.URL)
		}
		if b.Title != "New Title" {
			t.Errorf("expected Title 'New Title', got %q", b.Title)
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		err := db.UpdateBookmark(99999, "https://new.com", "New Title")
		if err == nil {
			t.Error("expected error for non-existent bookmark, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestDeleteBookmark tests deleting a bookmark.
func TestDeleteBookmark(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("deletes existing bookmark", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "To Delete")

		err := db.DeleteBookmark(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = db.GetBookmark(id)
		if err == nil {
			t.Error("expected error when getting deleted bookmark")
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		err := db.DeleteBookmark(99999)
		if err == nil {
			t.Error("expected error for non-existent bookmark, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestValidateBookmarkURL tests URL validation.
func TestValidateBookmarkURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{"valid http URL", "http://example.com", false, ""},
		{"valid https URL", "https://example.com", false, ""},
		{"valid URL with path", "https://example.com/path/to/page", false, ""},
		{"valid URL with query", "https://example.com?foo=bar", false, ""},
		{"valid URL with port", "https://example.com:8080/path", false, ""},
		{"empty URL", "", true, "empty URL"},
		{"no scheme", "example.com", true, "scheme must be http or https"},
		{"ftp scheme", "ftp://example.com", true, "scheme must be http or https"},
		{"javascript scheme", "javascript:alert(1)", true, "scheme must be http or https"},
		{"file scheme", "file:///etc/passwd", true, "scheme must be http or https"},
		{"missing host", "https://", true, "missing host"},
		{"data URI", "data:text/html,<h1>hi</h1>", true, "scheme must be http or https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBookmarkURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, ErrInvalidURL) {
					t.Errorf("expected ErrInvalidURL, got %v", err)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestAddBookmarkValidation tests that AddBookmark validates URLs.
func TestAddBookmarkValidation(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("rejects invalid URL", func(t *testing.T) {
		_, err := db.AddBookmark("not-a-url", "Invalid")
		if err == nil {
			t.Fatal("expected error for invalid URL, got nil")
		}
		if !errors.Is(err, ErrInvalidURL) {
			t.Errorf("expected ErrInvalidURL, got %v", err)
		}
	})

	t.Run("rejects ftp URL", func(t *testing.T) {
		_, err := db.AddBookmark("ftp://example.com", "FTP Site")
		if err == nil {
			t.Fatal("expected error for ftp URL, got nil")
		}
		if !errors.Is(err, ErrInvalidURL) {
			t.Errorf("expected ErrInvalidURL, got %v", err)
		}
	})

	t.Run("rejects empty URL", func(t *testing.T) {
		_, err := db.AddBookmark("", "No URL")
		if err == nil {
			t.Fatal("expected error for empty URL, got nil")
		}
		if !errors.Is(err, ErrInvalidURL) {
			t.Errorf("expected ErrInvalidURL, got %v", err)
		}
	})

	t.Run("accepts valid http URL", func(t *testing.T) {
		id, err := db.AddBookmark("http://example.com", "HTTP Site")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("accepts valid https URL", func(t *testing.T) {
		id, err := db.AddBookmark("https://example.com", "HTTPS Site")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})
}
