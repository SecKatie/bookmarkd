package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"time"
)

// ErrInvalidURL is returned when a bookmark URL fails validation.
var ErrInvalidURL = errors.New("invalid URL")

// ValidateBookmarkURL validates that a URL is acceptable for bookmarking.
// It requires the URL to have http or https scheme and a non-empty host.
func ValidateBookmarkURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("%w: empty URL", ErrInvalidURL)
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https, got %q", ErrInvalidURL, u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	return nil
}

// ------------------------------
// Bookmark methods
// ------------------------------

func (db *DB) GetBookmark(id int64) (Bookmark, error) {
	var b Bookmark
	err := db.db.QueryRow("SELECT id, url, title, created_at FROM bookmarks WHERE id = ?", id).
		Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark not found: %d", id)
		}
		return Bookmark{}, fmt.Errorf("failed to get bookmark: %w", err)
	}
	return b, nil
}

// AddBookmark adds a new bookmark to the database and returns the ID of the new bookmark.
//
// It validates the URL before inserting and returns ErrInvalidURL if validation fails.
// It returns the new bookmark ID (>0) on success.
// Emits a BookmarkCreatedEvent after successful insert.
func (db *DB) AddBookmark(url string, title string) (int64, error) {
	if err := ValidateBookmarkURL(url); err != nil {
		return 0, err
	}

	createdAt := time.Now().Format(time.RFC3339)
	result, err := db.db.Exec(
		"INSERT INTO bookmarks (url, title, created_at) VALUES (?, ?, ?)",
		url,
		title,
		createdAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add bookmark: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	db.emit(BookmarkCreatedEvent{
		Bookmark: Bookmark{
			ID:        id,
			URL:       url,
			Title:     title,
			CreatedAt: createdAt,
		},
	})

	return id, nil
}

func (db *DB) ListBookmarks(limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		ORDER BY created_at DESC
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = db.db.Query(query+" LIMIT ?", limit)
	} else {
		rows, err = db.db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list bookmarks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bookmark: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

// UpdateBookmark updates a bookmark's URL and title.
// Emits a BookmarkUpdatedEvent after successful update.
func (db *DB) UpdateBookmark(id int64, url string, title string) error {
	res, err := db.db.Exec("UPDATE bookmarks SET url = ?, title = ? WHERE id = ?", url, title, id)
	if err != nil {
		return fmt.Errorf("failed to update bookmark: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("bookmark not found: %d", id)
	}

	// Fetch the updated bookmark to emit in the event
	b, err := db.GetBookmark(id)
	if err == nil {
		db.emit(BookmarkUpdatedEvent{Bookmark: b})
	}

	return nil
}

// DeleteBookmark removes a bookmark from the database.
// Emits a BookmarkDeletedEvent after successful deletion.
func (db *DB) DeleteBookmark(id int64) error {
	// Fetch bookmark before deletion to include in event
	b, _ := db.GetBookmark(id)

	res, err := db.db.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete bookmark: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("bookmark not found: %d", id)
	}

	// If we couldn't fetch earlier, at least include the ID
	if b.ID == 0 {
		b.ID = id
	}
	db.emit(BookmarkDeletedEvent{Bookmark: b})

	return nil
}
