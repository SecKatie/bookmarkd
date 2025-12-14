package db

import (
	"errors"
	"testing"
	"time"
)

// TestEventKindString tests the String method on EventKind.
func TestEventKindString(t *testing.T) {
	tests := []struct {
		kind     EventKind
		expected string
	}{
		{OnBookmarkCreatedEvent, "bookmark_created"},
		{OnBookmarkDeletedEvent, "bookmark_deleted"},
		{OnBookmarkUpdatedEvent, "bookmark_updated"},
		{OnArchiveResultSavedEvent, "archive_result_saved"},
		{OnArchiveClearedEvent, "archive_cleared"},
		{EventKind(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestEventTypes tests that event types return correct Kind.
func TestEventTypes(t *testing.T) {
	t.Run("BookmarkCreatedEvent", func(t *testing.T) {
		e := BookmarkCreatedEvent{Bookmark: Bookmark{ID: 1}}
		if e.Kind() != OnBookmarkCreatedEvent {
			t.Errorf("expected OnBookmarkCreatedEvent, got %v", e.Kind())
		}
	})

	t.Run("BookmarkUpdatedEvent", func(t *testing.T) {
		e := BookmarkUpdatedEvent{Bookmark: Bookmark{ID: 1}}
		if e.Kind() != OnBookmarkUpdatedEvent {
			t.Errorf("expected OnBookmarkUpdatedEvent, got %v", e.Kind())
		}
	})

	t.Run("BookmarkDeletedEvent", func(t *testing.T) {
		e := BookmarkDeletedEvent{Bookmark: Bookmark{ID: 1}}
		if e.Kind() != OnBookmarkDeletedEvent {
			t.Errorf("expected OnBookmarkDeletedEvent, got %v", e.Kind())
		}
	})

	t.Run("ArchiveResultSavedEvent", func(t *testing.T) {
		e := ArchiveResultSavedEvent{BookmarkID: 1, Status: "ok"}
		if e.Kind() != OnArchiveResultSavedEvent {
			t.Errorf("expected OnArchiveResultSavedEvent, got %v", e.Kind())
		}
	})

	t.Run("ArchiveClearedEvent", func(t *testing.T) {
		e := ArchiveClearedEvent{BookmarkID: 1}
		if e.Kind() != OnArchiveClearedEvent {
			t.Errorf("expected OnArchiveClearedEvent, got %v", e.Kind())
		}
	})
}

// TestRegisterEventListener tests listener registration.
func TestRegisterEventListener(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	called := false
	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		called = true
		return nil
	})

	db.AddBookmark("https://example.com", "Test")

	if !called {
		t.Error("expected listener to be called")
	}
}

// TestBookmarkCreatedEvent tests that event is emitted on bookmark creation.
func TestBookmarkCreatedEvent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	var receivedEvent BookmarkCreatedEvent
	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		receivedEvent = event.(BookmarkCreatedEvent)
		return nil
	})

	id, _ := db.AddBookmark("https://example.com", "Test Site")

	if receivedEvent.Bookmark.ID != id {
		t.Errorf("expected bookmark ID %d, got %d", id, receivedEvent.Bookmark.ID)
	}
	if receivedEvent.Bookmark.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %q", receivedEvent.Bookmark.URL)
	}
	if receivedEvent.Bookmark.Title != "Test Site" {
		t.Errorf("expected Title 'Test Site', got %q", receivedEvent.Bookmark.Title)
	}
}

// TestBookmarkUpdatedEvent tests that event is emitted on bookmark update.
func TestBookmarkUpdatedEvent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	id, _ := db.AddBookmark("https://old.com", "Old Title")

	var receivedEvent BookmarkUpdatedEvent
	db.RegisterEventListener(OnBookmarkUpdatedEvent, func(event Event) error {
		receivedEvent = event.(BookmarkUpdatedEvent)
		return nil
	})

	db.UpdateBookmark(id, "https://new.com", "New Title")

	if receivedEvent.Bookmark.ID != id {
		t.Errorf("expected bookmark ID %d, got %d", id, receivedEvent.Bookmark.ID)
	}
	if receivedEvent.Bookmark.URL != "https://new.com" {
		t.Errorf("expected URL 'https://new.com', got %q", receivedEvent.Bookmark.URL)
	}
}

// TestBookmarkDeletedEvent tests that event is emitted on bookmark deletion.
func TestBookmarkDeletedEvent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	id, _ := db.AddBookmark("https://example.com", "To Delete")

	var receivedEvent BookmarkDeletedEvent
	db.RegisterEventListener(OnBookmarkDeletedEvent, func(event Event) error {
		receivedEvent = event.(BookmarkDeletedEvent)
		return nil
	})

	db.DeleteBookmark(id)

	if receivedEvent.Bookmark.ID != id {
		t.Errorf("expected bookmark ID %d, got %d", id, receivedEvent.Bookmark.ID)
	}
}

// TestArchiveResultSavedEvent tests that event is emitted when archive result is saved.
func TestArchiveResultSavedEvent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	id, _ := db.AddBookmark("https://example.com", "Test")

	var receivedEvent ArchiveResultSavedEvent
	db.RegisterEventListener(OnArchiveResultSavedEvent, func(event Event) error {
		receivedEvent = event.(ArchiveResultSavedEvent)
		return nil
	})

	now := time.Now()
	db.SaveArchiveResult(id, now, &now, "ok", "", "", "<html></html>")

	if receivedEvent.BookmarkID != id {
		t.Errorf("expected bookmark ID %d, got %d", id, receivedEvent.BookmarkID)
	}
	if receivedEvent.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", receivedEvent.Status)
	}
}

// TestArchiveClearedEvent tests that event is emitted when archive is cleared.
func TestArchiveClearedEvent(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	id, _ := db.AddBookmark("https://example.com", "Test")
	now := time.Now()
	db.SaveArchiveResult(id, now, &now, "ok", "", "", "<html></html>")

	var receivedEvent ArchiveClearedEvent
	db.RegisterEventListener(OnArchiveClearedEvent, func(event Event) error {
		receivedEvent = event.(ArchiveClearedEvent)
		return nil
	})

	db.ClearBookmarkArchive(id)

	if receivedEvent.BookmarkID != id {
		t.Errorf("expected bookmark ID %d, got %d", id, receivedEvent.BookmarkID)
	}
}

// TestMultipleListeners tests that multiple listeners are called.
func TestMultipleListeners(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	callCount := 0

	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		callCount++
		return nil
	})
	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		callCount++
		return nil
	})

	db.AddBookmark("https://example.com", "Test")

	if callCount != 2 {
		t.Errorf("expected 2 listeners to be called, got %d", callCount)
	}
}

// TestListenerErrors tests that listener errors are handled gracefully.
func TestListenerErrors(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	secondCalled := false

	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		return errors.New("first listener error")
	})
	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		secondCalled = true
		return nil
	})

	// Should not panic and should continue to next listener
	id, err := db.AddBookmark("https://example.com", "Test")
	if err != nil {
		t.Fatalf("expected no error from AddBookmark, got %v", err)
	}
	if id <= 0 {
		t.Error("expected valid bookmark ID")
	}
	if !secondCalled {
		t.Error("expected second listener to be called despite first listener error")
	}
}

// TestListenersForDifferentEvents tests that listeners only receive their event type.
func TestListenersForDifferentEvents(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	createdCalled := false
	deletedCalled := false

	db.RegisterEventListener(OnBookmarkCreatedEvent, func(event Event) error {
		createdCalled = true
		return nil
	})
	db.RegisterEventListener(OnBookmarkDeletedEvent, func(event Event) error {
		deletedCalled = true
		return nil
	})

	// Only create a bookmark, don't delete
	db.AddBookmark("https://example.com", "Test")

	if !createdCalled {
		t.Error("expected created listener to be called")
	}
	if deletedCalled {
		t.Error("expected deleted listener NOT to be called")
	}
}
