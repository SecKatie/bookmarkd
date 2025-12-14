package db

import (
	"database/sql"
	"errors"
	"fmt"
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

func (db *DB) ListBookmarksToArchive(limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archived_at IS NULL
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
		return nil, fmt.Errorf("failed to list bookmarks to archive: %w", err)
	}
	defer rows.Close()

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

func (db *DB) ListArchivedBookmarks(limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archived_at IS NOT NULL
		ORDER BY archived_at DESC
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = db.db.Query(query+" LIMIT ?", limit)
	} else {
		rows, err = db.db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list archived bookmarks: %w", err)
	}
	defer rows.Close()

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

func (db *DB) ListBookmarksByArchiveStatus(status string, limit int) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, created_at
		FROM bookmarks
		WHERE archive_status = ?
		ORDER BY archive_attempted_at DESC
	`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = db.db.Query(query+" LIMIT ?", status, limit)
	} else {
		rows, err = db.db.Query(query, status)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list bookmarks by archive status: %w", err)
	}
	defer rows.Close()

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
