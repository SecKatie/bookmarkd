package db

import "log"

// ------------------------------
// Event System
// ------------------------------
//
// The DB emits typed events when bookmarks are created, updated, deleted,
// or when archive results are saved. Register listeners to react to these changes.
//
// Example usage:
//
//	db.RegisterEventListener(db.OnBookmarkCreatedEvent, func(event db.Event) error {
//	    ev := event.(db.BookmarkCreatedEvent)
//	    log.Printf("New bookmark created: %d - %s", ev.Bookmark.ID, ev.Bookmark.URL)
//	    // Optionally queue an archive job here
//	    return nil
//	})
//
//	db.RegisterEventListener(db.OnBookmarkUpdatedEvent, func(event db.Event) error {
//	    ev := event.(db.BookmarkUpdatedEvent)
//	    log.Printf("Bookmark updated: %d", ev.Bookmark.ID)
//	    return nil
//	})
//
// Event is the common interface for all database events.
type Event interface {
	Kind() EventKind
}

// EventKind represents all the kinds of events that can be emitted by the DB.
type EventKind int

const (
	// OnBookmarkCreatedEvent is emitted when a bookmark is created.
	OnBookmarkCreatedEvent EventKind = iota
	// OnBookmarkDeletedEvent is emitted when a bookmark is deleted.
	OnBookmarkDeletedEvent
	// OnBookmarkUpdatedEvent is emitted when a bookmark is updated.
	OnBookmarkUpdatedEvent
	// OnArchiveResultSavedEvent is emitted when an archive result is saved.
	OnArchiveResultSavedEvent
	// OnArchiveClearedEvent is emitted when an archive is cleared for re-archiving.
	OnArchiveClearedEvent
)

func (k EventKind) String() string {
	switch k {
	case OnBookmarkCreatedEvent:
		return "bookmark_created"
	case OnBookmarkDeletedEvent:
		return "bookmark_deleted"
	case OnBookmarkUpdatedEvent:
		return "bookmark_updated"
	case OnArchiveResultSavedEvent:
		return "archive_result_saved"
	case OnArchiveClearedEvent:
		return "archive_cleared"
	default:
		return "unknown"
	}
}

// BookmarkCreatedEvent is emitted after a new bookmark is successfully inserted.
type BookmarkCreatedEvent struct {
	Bookmark Bookmark
}

func (e BookmarkCreatedEvent) Kind() EventKind { return OnBookmarkCreatedEvent }

// BookmarkUpdatedEvent is emitted after a bookmark's URL or title is updated.
type BookmarkUpdatedEvent struct {
	Bookmark Bookmark
}

func (e BookmarkUpdatedEvent) Kind() EventKind { return OnBookmarkUpdatedEvent }

// BookmarkDeletedEvent is emitted after a bookmark is deleted.
// The Bookmark field contains the state before deletion (if available).
type BookmarkDeletedEvent struct {
	Bookmark Bookmark
}

func (e BookmarkDeletedEvent) Kind() EventKind { return OnBookmarkDeletedEvent }

// ArchiveResultSavedEvent is emitted after an archive result is saved.
type ArchiveResultSavedEvent struct {
	BookmarkID int64
	Status     string // "ok" or "error"
}

func (e ArchiveResultSavedEvent) Kind() EventKind { return OnArchiveResultSavedEvent }

// ArchiveClearedEvent is emitted after an archive is cleared for re-archiving.
type ArchiveClearedEvent struct {
	BookmarkID int64
}

func (e ArchiveClearedEvent) Kind() EventKind { return OnArchiveClearedEvent }

// EventListener is a callback that handles events of a specific kind.
type EventListener func(event Event) error

// RegisterEventListener adds a listener for a specific event kind.
// Listeners are called synchronously in registration order after the DB operation succeeds.
func (db *DB) RegisterEventListener(eventKind EventKind, listener EventListener) {
	if db.eventListeners == nil {
		db.eventListeners = make(map[EventKind][]EventListener)
	}
	db.eventListeners[eventKind] = append(db.eventListeners[eventKind], listener)
}

// emit dispatches an event to all registered listeners for that event kind.
func (db *DB) emit(event Event) {
	listeners := db.eventListeners[event.Kind()]
	for _, listener := range listeners {
		if err := listener(event); err != nil {
			log.Printf("Event listener error for %s: %v", event.Kind(), err)
		}
	}
}
