package web

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/seckatie/bookmarkd/internal/core"
	"github.com/seckatie/bookmarkd/internal/core/db"
)

// handleArchive routes archive-related requests
func (ws *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse bookmark ID from URL: /bookmarks/{id}/archive or /bookmarks/{id}/archive/raw
	path := strings.TrimPrefix(r.URL.Path, "/bookmarks/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid bookmark ID", http.StatusBadRequest)
		return
	}

	// Check if this is a raw request
	if len(parts) >= 3 && parts[2] == "raw" {
		ws.serveArchiveHTML(w, r, id)
		return
	}

	ws.viewArchive(w, r, id)
}

// viewArchive renders the archive viewer page with iframe
func (ws *Server) viewArchive(w http.ResponseWriter, _ *http.Request, id int64) {
	bookmark, err := ws.db.GetBookmark(id)
	if err != nil {
		http.Error(w, "Bookmark not found", http.StatusNotFound)
		return
	}

	archive, err := ws.db.GetBookmarkArchive(id)
	if err != nil || archive.ArchiveStatus != core.ArchiveStatusOK {
		http.Error(w, "Archive not available", http.StatusNotFound)
		return
	}

	view := map[string]any{
		"ID":         bookmark.ID,
		"URL":        bookmark.URL,
		"Title":      bookmark.Title,
		"RawURL":     fmt.Sprintf("/bookmarks/%d/archive/raw", id),
		"ActivePage": "archives",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ws.templates.ExecuteTemplate(w, "viewer.html", view); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute viewer template: %v", err)
		return
	}
}

// serveArchiveHTML serves the raw archived HTML content
func (ws *Server) serveArchiveHTML(w http.ResponseWriter, _ *http.Request, id int64) {
	archive, err := ws.db.GetBookmarkArchive(id)
	if err != nil {
		http.Error(w, "Bookmark not found", http.StatusNotFound)
		return
	}

	if archive.ArchiveStatus != core.ArchiveStatusOK || archive.ArchivedHTML == "" {
		http.Error(w, "Archive not available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(archive.ArchivedHTML)); err != nil {
		log.Printf("Failed to write archived HTML: %v", err)
	}
}

// handleArchiveManager serves the archive manager page
func (ws *Server) handleArchiveManager(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{"ActivePage": "archives"}
	if err := ws.templates.ExecuteTemplate(w, "archives.html", data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute archives template: %v", err)
	}
}

// buildArchiveManagerView builds an archiveManagerView from a bookmark
func (ws *Server) buildArchiveManagerView(b db.Bookmark) archiveManagerView {
	view := archiveManagerView{
		ID:    b.ID,
		URL:   b.URL,
		Title: b.Title,
	}
	archive, err := ws.db.GetBookmarkArchive(b.ID)
	if err == nil {
		view.ArchiveStatus = archive.ArchiveStatus
		view.ArchivedAt = archive.ArchivedAt
		view.ArchiveAttemptedAt = archive.ArchiveAttemptedAt
		view.ArchiveError = archive.ArchiveError
		// IsArchiving is true when there's no archived_at (queued/in-progress)
		// but not when it's an error state
		view.IsArchiving = archive.ArchivedAt == "" && archive.ArchiveStatus != core.ArchiveStatusError
	} else {
		// If we can't get archive info, assume it needs archiving
		view.IsArchiving = true
	}
	return view
}

// handleArchivesList serves the archives list fragment
func (ws *Server) handleArchivesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	bookmarks, err := ws.db.ListBookmarks(0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to get bookmarks: %v", err)
		return
	}

	var archivesData []archiveManagerView
	for _, b := range bookmarks {
		archivesData = append(archivesData, ws.buildArchiveManagerView(b))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ws.templates.ExecuteTemplate(w, "archives_list.html", map[string]any{"archives": archivesData}); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute archives list template: %v", err)
		return
	}
}

// handleArchivesRoutes routes archive management requests
func (ws *Server) handleArchivesRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/archives/")

	// Handle /archives/list
	if path == "list" {
		ws.handleArchivesList(w, r)
		return
	}

	// Handle /archives/{id}/refetch and /archives/{id}/status
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "Invalid bookmark ID", http.StatusBadRequest)
			return
		}

		switch parts[1] {
		case "refetch":
			if r.Method != http.MethodPost {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			ws.refetchArchive(w, r, id)
			return
		case "status":
			if r.Method != http.MethodGet {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			ws.getArchiveItemStatus(w, r, id)
			return
		}
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

// getArchiveItemStatus returns the current status of a single archive item
func (ws *Server) getArchiveItemStatus(w http.ResponseWriter, r *http.Request, id int64) {
	bookmark, err := ws.db.GetBookmark(id)
	if err != nil {
		http.Error(w, "Bookmark not found", http.StatusNotFound)
		log.Printf("Failed to get bookmark %d: %v", id, err)
		return
	}

	view := ws.buildArchiveManagerView(bookmark)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ws.templates.ExecuteTemplate(w, "archive_item.html", view); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute archive item template: %v", err)
		return
	}
}

// refetchArchive clears an existing archive to queue it for re-archiving
func (ws *Server) refetchArchive(w http.ResponseWriter, r *http.Request, id int64) {
	bookmark, err := ws.db.GetBookmark(id)
	if err != nil {
		http.Error(w, "Bookmark not found", http.StatusNotFound)
		log.Printf("Failed to get bookmark %d: %v", id, err)
		return
	}

	if err := ws.db.ClearBookmarkArchive(id); err != nil {
		http.Error(w, "Failed to clear archive", http.StatusInternalServerError)
		log.Printf("Failed to clear bookmark archive %d: %v", id, err)
		return
	}

	log.Printf("Cleared archive for bookmark %d, queued for re-archiving", id)

	// For HTMX requests, return just the single item in archiving state
	if r.Header.Get("HX-Request") == "true" {
		view := ws.buildArchiveManagerView(bookmark)
		// Force IsArchiving to true since we just cleared it
		view.IsArchiving = true
		view.ArchiveStatus = ""
		view.ArchivedAt = ""
		view.ArchiveError = ""

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ws.templates.ExecuteTemplate(w, "archive_item.html", view); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Failed to execute archive item template: %v", err)
			return
		}
		return
	}

	http.Redirect(w, r, "/archives", http.StatusSeeOther)
}
