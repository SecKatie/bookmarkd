package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"
)

func (db *DB) QueueBookmarkForArchive(id int64) error {
	_, err := db.db.Exec(`
		UPDATE bookmarks
		SET archived_at = NULL
		WHERE id = ?
	`, id)
	return err
}

// scanBookmarks extracts Bookmark structs from SQL rows.
// This is a helper to reduce duplication across bookmark query functions.
func scanBookmarks(rows *sql.Rows) ([]Bookmark, error) {
	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bookmark: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookmark rows: %w", err)
	}
	return out, nil
}

// queryBookmarks executes a bookmark query with optional limit and returns the results.
// This is a helper to reduce duplication across list functions.
func (db *DB) queryBookmarks(query string, args []any, limit int) ([]Bookmark, error) {
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	return scanBookmarks(rows)
}

func (db *DB) ListBookmarksToArchive(limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archived_at IS NULL
		ORDER BY created_at DESC`
	bookmarks, err := db.queryBookmarks(query, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list bookmarks to archive: %w", err)
	}
	return bookmarks, nil
}

func (db *DB) ListArchivedBookmarks(limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archived_at IS NOT NULL
		ORDER BY archived_at DESC`
	bookmarks, err := db.queryBookmarks(query, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list archived bookmarks: %w", err)
	}
	return bookmarks, nil
}

func (db *DB) ListBookmarksByArchiveStatus(status string, limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archive_status = ?
		ORDER BY archive_attempted_at DESC`
	bookmarks, err := db.queryBookmarks(query, []any{status}, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list bookmarks by archive status: %w", err)
	}
	return bookmarks, nil
}

func (db *DB) GetBookmarkArchive(id int64) (BookmarkArchive, error) {
	var a BookmarkArchive
	err := db.db.QueryRow(`
		SELECT
			id,
			COALESCE(archived_url, ''),
			COALESCE(archived_html, ''),
			COALESCE(archive_attempted_at, ''),
			COALESCE(archived_at, ''),
			COALESCE(archive_status, ''),
			COALESCE(archive_error, '')
		FROM bookmarks
		WHERE id = ?
	`, id).Scan(
		&a.BookmarkID,
		&a.ArchivedURL,
		&a.ArchivedHTML,
		&a.ArchiveAttemptedAt,
		&a.ArchivedAt,
		&a.ArchiveStatus,
		&a.ArchiveError,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BookmarkArchive{}, fmt.Errorf("bookmark not found: %d", id)
		}
		return BookmarkArchive{}, fmt.Errorf("failed to get bookmark archive: %w", err)
	}
	return a, nil
}

func (db *DB) ClearBookmarkArchive(id int64) error {
	res, err := db.db.Exec(`
		UPDATE bookmarks
		SET
			archived_html = NULL,
			archived_url = NULL,
			archive_attempted_at = NULL,
			archived_at = NULL,
			archive_status = NULL,
			archive_error = NULL
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("failed to clear bookmark archive: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("bookmark not found: %d", id)
	}

	// Emit event so bookmark can be queued for re-archiving
	db.emit(ArchiveClearedEvent{
		BookmarkID: id,
	})

	return nil
}

// SaveArchiveResult saves the result of an archive operation.
// Emits an ArchiveResultSavedEvent after successful save.
func (db *DB) SaveArchiveResult(id int64, attemptedAt time.Time, archivedAt *time.Time, status string, archiveErr string, archivedURL string, archivedHTML string) error {
	var archivedAtStr any = nil
	if archivedAt != nil {
		archivedAtStr = archivedAt.Format(time.RFC3339)
	}

	res, err := db.db.Exec(`
		UPDATE bookmarks
		SET
			archive_attempted_at = ?,
			archived_at = ?,
			archive_status = ?,
			archive_error = ?,
			archived_url = ?,
			archived_html = ?
		WHERE id = ?
	`,
		attemptedAt.Format(time.RFC3339),
		archivedAtStr,
		status,
		archiveErr,
		archivedURL,
		archivedHTML,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to save archive result: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("bookmark not found: %d", id)
	}

	db.emit(ArchiveResultSavedEvent{
		BookmarkID: id,
		Status:     status,
	})

	return nil
}
