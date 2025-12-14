package db

import (
	"strings"
	"testing"
	"time"
)

// TestListBookmarksToArchive tests listing bookmarks that need archiving.
func TestListBookmarksToArchive(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("returns all unarchived bookmarks", func(t *testing.T) {
		db.AddBookmark("https://site1.com", "Site 1")
		db.AddBookmark("https://site2.com", "Site 2")

		bookmarks, err := db.ListBookmarksToArchive(0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 2 {
			t.Errorf("expected 2 bookmarks to archive, got %d", len(bookmarks))
		}
	})

	t.Run("excludes archived bookmarks", func(t *testing.T) {
		// Create a fresh database
		db2 := newTestDB(t)
		defer db2.Close()

		id1, _ := db2.AddBookmark("https://archived.com", "Archived")
		db2.AddBookmark("https://unarchived.com", "Unarchived")

		// Archive the first one
		now := time.Now()
		db2.SaveArchiveResult(id1, now, &now, "ok", "", "https://archived.com", "<html></html>")

		bookmarks, err := db2.ListBookmarksToArchive(0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 1 {
			t.Errorf("expected 1 bookmark to archive, got %d", len(bookmarks))
		}
		if bookmarks[0].URL != "https://unarchived.com" {
			t.Errorf("expected unarchived bookmark, got %q", bookmarks[0].URL)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		db3 := newTestDB(t)
		defer db3.Close()

		db3.AddBookmark("https://site1.com", "Site 1")
		db3.AddBookmark("https://site2.com", "Site 2")
		db3.AddBookmark("https://site3.com", "Site 3")

		bookmarks, err := db3.ListBookmarksToArchive(2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 2 {
			t.Errorf("expected 2 bookmarks with limit, got %d", len(bookmarks))
		}
	})
}

// TestListArchivedBookmarks tests listing successfully archived bookmarks.
func TestListArchivedBookmarks(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("returns only archived bookmarks", func(t *testing.T) {
		id1, _ := db.AddBookmark("https://archived.com", "Archived")
		db.AddBookmark("https://unarchived.com", "Unarchived")

		now := time.Now()
		db.SaveArchiveResult(id1, now, &now, "ok", "", "https://archived.com", "<html></html>")

		bookmarks, err := db.ListArchivedBookmarks(0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 1 {
			t.Errorf("expected 1 archived bookmark, got %d", len(bookmarks))
		}
		if bookmarks[0].URL != "https://archived.com" {
			t.Errorf("expected archived bookmark, got %q", bookmarks[0].URL)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		db2 := newTestDB(t)
		defer db2.Close()

		id1, _ := db2.AddBookmark("https://site1.com", "Site 1")
		id2, _ := db2.AddBookmark("https://site2.com", "Site 2")
		id3, _ := db2.AddBookmark("https://site3.com", "Site 3")

		now := time.Now()
		db2.SaveArchiveResult(id1, now, &now, "ok", "", "", "<html>1</html>")
		db2.SaveArchiveResult(id2, now, &now, "ok", "", "", "<html>2</html>")
		db2.SaveArchiveResult(id3, now, &now, "ok", "", "", "<html>3</html>")

		bookmarks, err := db2.ListArchivedBookmarks(2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 2 {
			t.Errorf("expected 2 archived bookmarks, got %d", len(bookmarks))
		}
	})
}

// TestListBookmarksByArchiveStatus tests filtering by archive status.
func TestListBookmarksByArchiveStatus(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	id1, _ := db.AddBookmark("https://success.com", "Success")
	id2, _ := db.AddBookmark("https://error.com", "Error")
	db.AddBookmark("https://pending.com", "Pending")

	now := time.Now()
	db.SaveArchiveResult(id1, now, &now, "ok", "", "", "<html></html>")
	db.SaveArchiveResult(id2, now, nil, "error", "connection timeout", "", "")

	t.Run("filters by ok status", func(t *testing.T) {
		bookmarks, err := db.ListBookmarksByArchiveStatus("ok", 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 1 {
			t.Errorf("expected 1 bookmark with ok status, got %d", len(bookmarks))
		}
		if bookmarks[0].URL != "https://success.com" {
			t.Errorf("expected success bookmark, got %q", bookmarks[0].URL)
		}
	})

	t.Run("filters by error status", func(t *testing.T) {
		bookmarks, err := db.ListBookmarksByArchiveStatus("error", 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(bookmarks) != 1 {
			t.Errorf("expected 1 bookmark with error status, got %d", len(bookmarks))
		}
		if bookmarks[0].URL != "https://error.com" {
			t.Errorf("expected error bookmark, got %q", bookmarks[0].URL)
		}
	})
}

// TestGetBookmarkArchive tests retrieving archive data.
func TestGetBookmarkArchive(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("retrieves archive for existing bookmark", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "Example")

		now := time.Now()
		html := "<html><body>Hello</body></html>"
		db.SaveArchiveResult(id, now, &now, "ok", "", "https://example.com/final", html)

		archive, err := db.GetBookmarkArchive(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if archive.BookmarkID != id {
			t.Errorf("expected BookmarkID %d, got %d", id, archive.BookmarkID)
		}
		if archive.ArchiveStatus != "ok" {
			t.Errorf("expected status 'ok', got %q", archive.ArchiveStatus)
		}
		if archive.ArchivedURL != "https://example.com/final" {
			t.Errorf("expected archived URL, got %q", archive.ArchivedURL)
		}
		if archive.ArchivedHTML != html {
			t.Errorf("expected HTML content, got %q", archive.ArchivedHTML)
		}
	})

	t.Run("returns empty fields for unarchived bookmark", func(t *testing.T) {
		id, _ := db.AddBookmark("https://new.com", "New")

		archive, err := db.GetBookmarkArchive(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if archive.ArchiveStatus != "" {
			t.Errorf("expected empty status, got %q", archive.ArchiveStatus)
		}
		if archive.ArchivedHTML != "" {
			t.Errorf("expected empty HTML, got %q", archive.ArchivedHTML)
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		_, err := db.GetBookmarkArchive(99999)
		if err == nil {
			t.Error("expected error for non-existent bookmark")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestSaveArchiveResult tests saving archive results.
func TestSaveArchiveResult(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("saves successful archive", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "Example")

		attemptedAt := time.Now()
		archivedAt := attemptedAt.Add(5 * time.Second)
		html := "<html></html>"

		err := db.SaveArchiveResult(id, attemptedAt, &archivedAt, "ok", "", "https://example.com", html)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		archive, _ := db.GetBookmarkArchive(id)
		if archive.ArchiveStatus != "ok" {
			t.Errorf("expected status 'ok', got %q", archive.ArchiveStatus)
		}
		if archive.ArchiveError != "" {
			t.Errorf("expected no error, got %q", archive.ArchiveError)
		}
	})

	t.Run("saves failed archive", func(t *testing.T) {
		id, _ := db.AddBookmark("https://fail.com", "Fail")

		attemptedAt := time.Now()
		errMsg := "connection refused"

		err := db.SaveArchiveResult(id, attemptedAt, nil, "error", errMsg, "", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		archive, _ := db.GetBookmarkArchive(id)
		if archive.ArchiveStatus != "error" {
			t.Errorf("expected status 'error', got %q", archive.ArchiveStatus)
		}
		if archive.ArchiveError != errMsg {
			t.Errorf("expected error %q, got %q", errMsg, archive.ArchiveError)
		}
		if archive.ArchivedAt != "" {
			t.Errorf("expected empty archived_at for failed archive, got %q", archive.ArchivedAt)
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		err := db.SaveArchiveResult(99999, time.Now(), nil, "ok", "", "", "")
		if err == nil {
			t.Error("expected error for non-existent bookmark")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestClearBookmarkArchive tests clearing archive data.
func TestClearBookmarkArchive(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("clears existing archive", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "Example")

		now := time.Now()
		db.SaveArchiveResult(id, now, &now, "ok", "", "https://example.com", "<html></html>")

		err := db.ClearBookmarkArchive(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		archive, _ := db.GetBookmarkArchive(id)
		if archive.ArchiveStatus != "" {
			t.Errorf("expected empty status after clear, got %q", archive.ArchiveStatus)
		}
		if archive.ArchivedHTML != "" {
			t.Errorf("expected empty HTML after clear, got %q", archive.ArchivedHTML)
		}
	})

	t.Run("returns error for non-existent bookmark", func(t *testing.T) {
		err := db.ClearBookmarkArchive(99999)
		if err == nil {
			t.Error("expected error for non-existent bookmark")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})
}

// TestQueueBookmarkForArchive tests queueing a bookmark for archive.
func TestQueueBookmarkForArchive(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	t.Run("queues archived bookmark for re-archive", func(t *testing.T) {
		id, _ := db.AddBookmark("https://example.com", "Example")

		now := time.Now()
		db.SaveArchiveResult(id, now, &now, "ok", "", "", "<html></html>")

		// Verify it's archived
		archived, _ := db.ListArchivedBookmarks(0)
		if len(archived) != 1 {
			t.Fatalf("expected 1 archived bookmark, got %d", len(archived))
		}

		// Queue for re-archive
		err := db.QueueBookmarkForArchive(id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Should now appear in "to archive" list
		toArchive, _ := db.ListBookmarksToArchive(0)
		if len(toArchive) != 1 {
			t.Errorf("expected 1 bookmark to archive after queue, got %d", len(toArchive))
		}
	})
}
