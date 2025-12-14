package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestHandleIndex tests the index page handler.
func TestHandleIndex(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET returns index page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.handleIndex(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("expected Content-Type 'text/html; charset=utf-8', got %q", ct)
		}
		if !strings.Contains(w.Body.String(), "bookmarkd") {
			t.Error("expected response to contain 'bookmarkd'")
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		w := httptest.NewRecorder()

		server.handleIndex(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleBookmarklet tests the bookmarklet page handler.
func TestHandleBookmarklet(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET returns bookmarklet page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarklet", nil)
		w := httptest.NewRecorder()

		server.handleBookmarklet(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("expected Content-Type 'text/html; charset=utf-8', got %q", ct)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/bookmarklet", nil)
		w := httptest.NewRecorder()

		server.handleBookmarklet(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleBookmarkletAdd tests the bookmarklet add handler.
func TestHandleBookmarkletAdd(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET with url and title", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarklet/add?url=https://example.com&title=Example", nil)
		w := httptest.NewRecorder()

		server.handleBookmarkletAdd(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "https://example.com") {
			t.Error("expected response to contain URL")
		}
		if !strings.Contains(body, "Example") {
			t.Error("expected response to contain title")
		}
	})

	t.Run("GET with url only uses url as title", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarklet/add?url=https://example.com", nil)
		w := httptest.NewRecorder()

		server.handleBookmarkletAdd(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("GET without url returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarklet/add", nil)
		w := httptest.NewRecorder()

		server.handleBookmarkletAdd(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/bookmarklet/add?url=https://example.com", nil)
		w := httptest.NewRecorder()

		server.handleBookmarkletAdd(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleBookmarks tests the bookmarks list/create handler.
func TestHandleBookmarks(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET returns bookmarks list", func(t *testing.T) {
		// Add a bookmark first
		if _, err := server.db.AddBookmark("https://example.com", "Example Site"); err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/bookmarks", nil)
		w := httptest.NewRecorder()

		server.handleBookmarks(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "example.com") {
			t.Error("expected response to contain bookmark URL")
		}
	})

	t.Run("POST creates bookmark and redirects", func(t *testing.T) {
		form := url.Values{}
		form.Add("url", "https://newsite.com")
		form.Add("title", "New Site")

		req := httptest.NewRequest(http.MethodPost, "/bookmarks", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleBookmarks(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/" {
			t.Errorf("expected redirect to '/', got %q", loc)
		}
	})

	t.Run("POST with HX-Request returns list fragment", func(t *testing.T) {
		form := url.Values{}
		form.Add("url", "https://htmxsite.com")
		form.Add("title", "HTMX Site")

		req := httptest.NewRequest(http.MethodPost, "/bookmarks", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		server.handleBookmarks(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		// Should return HTML fragment, not redirect
		if w.Header().Get("Location") != "" {
			t.Error("expected no redirect for HTMX request")
		}
	})

	t.Run("DELETE returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/bookmarks", nil)
		w := httptest.NewRecorder()

		server.handleBookmarks(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleArchive tests the archive viewer handler.
func TestHandleArchive(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET with invalid path returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarks/", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("GET with invalid ID returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarks/abc/archive", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("GET archive for non-existent bookmark returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bookmarks/99999/archive", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("GET archive for bookmark without archive returns not found", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://example.com", "Example")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/bookmarks/"+itoa(id)+"/archive", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("GET archive for archived bookmark returns viewer", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://archived.com", "Archived Site")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		if err := server.db.SaveArchiveResult(id, now, &now, "ok", "", "https://archived.com", "<html><body>Archived</body></html>"); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/bookmarks/"+itoa(id)+"/archive", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Archived Site") {
			t.Error("expected response to contain bookmark title")
		}
	})

	t.Run("GET raw archive returns HTML content", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://raw.com", "Raw Site")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		htmlContent := "<html><body>Raw HTML Content</body></html>"
		if err := server.db.SaveArchiveResult(id, now, &now, "ok", "", "https://raw.com", htmlContent); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/bookmarks/"+itoa(id)+"/archive/raw", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if w.Body.String() != htmlContent {
			t.Errorf("expected raw HTML content, got %q", w.Body.String())
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/bookmarks/1/archive", nil)
		w := httptest.NewRecorder()

		server.handleArchive(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleArchiveManager tests the archive manager page handler.
func TestHandleArchiveManager(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET returns archive manager page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/archives", nil)
		w := httptest.NewRecorder()

		server.handleArchiveManager(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("expected Content-Type 'text/html; charset=utf-8', got %q", ct)
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/archives", nil)
		w := httptest.NewRecorder()

		server.handleArchiveManager(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleArchivesList tests the archives list fragment handler.
func TestHandleArchivesList(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("GET returns archives list", func(t *testing.T) {
		if _, err := server.db.AddBookmark("https://example.com", "Example"); err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/archives/list", nil)
		w := httptest.NewRecorder()

		server.handleArchivesList(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "example.com") {
			t.Error("expected response to contain bookmark URL")
		}
	})

	t.Run("POST returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/archives/list", nil)
		w := httptest.NewRecorder()

		server.handleArchivesList(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

// TestHandleArchivesRoutes tests the archives routing handler.
func TestHandleArchivesRoutes(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("routes to list handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/archives/list", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("routes to status handler", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://example.com", "Example")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/archives/"+itoa(id)+"/status", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("status for non-existent bookmark returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/archives/99999/status", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("refetch requires POST", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://example.com", "Example")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/archives/"+itoa(id)+"/refetch", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})

	t.Run("invalid path returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/archives/invalid/path", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("unknown route returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/archives/1/unknown", nil)
		w := httptest.NewRecorder()

		server.handleArchivesRoutes(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

// TestRefetchArchive tests the refetch archive handler.
func TestRefetchArchive(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("POST clears archive and redirects", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://example.com", "Example")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		if err := server.db.SaveArchiveResult(id, now, &now, "ok", "", "https://example.com", "<html></html>"); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/archives/"+itoa(id)+"/refetch", nil)
		w := httptest.NewRecorder()

		server.refetchArchive(w, req, id)

		if w.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, w.Code)
		}

		// Verify archive was cleared
		archive, _ := server.db.GetBookmarkArchive(id)
		if archive.ArchiveStatus != "" {
			t.Error("expected archive to be cleared")
		}
	})

	t.Run("POST with HX-Request returns item fragment", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://htmx.com", "HTMX")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		if err := server.db.SaveArchiveResult(id, now, &now, "ok", "", "https://htmx.com", "<html></html>"); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/archives/"+itoa(id)+"/refetch", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		server.refetchArchive(w, req, id)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		// Should return HTML fragment, not redirect
		if w.Header().Get("Location") != "" {
			t.Error("expected no redirect for HTMX request")
		}
	})

	t.Run("POST for non-existent bookmark returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/archives/99999/refetch", nil)
		w := httptest.NewRecorder()

		server.refetchArchive(w, req, 99999)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

// TestBuildArchiveManagerView tests the view builder function.
func TestBuildArchiveManagerView(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() {
		if err := server.db.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})

	t.Run("builds view for unarchived bookmark", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://example.com", "Example")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		bookmark, err := server.db.GetBookmark(id)
		if err != nil {
			t.Fatalf("failed to get bookmark: %v", err)
		}

		view := server.buildArchiveManagerView(bookmark)

		if view.ID != id {
			t.Errorf("expected ID %d, got %d", id, view.ID)
		}
		if view.URL != "https://example.com" {
			t.Errorf("expected URL 'https://example.com', got %q", view.URL)
		}
		if view.Title != "Example" {
			t.Errorf("expected Title 'Example', got %q", view.Title)
		}
		if !view.IsArchiving {
			t.Error("expected IsArchiving to be true for unarchived bookmark")
		}
	})

	t.Run("builds view for archived bookmark", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://archived.com", "Archived")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		if err := server.db.SaveArchiveResult(id, now, &now, "ok", "", "https://archived.com", "<html></html>"); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}
		bookmark, err := server.db.GetBookmark(id)
		if err != nil {
			t.Fatalf("failed to get bookmark: %v", err)
		}

		view := server.buildArchiveManagerView(bookmark)

		if view.ArchiveStatus != "ok" {
			t.Errorf("expected ArchiveStatus 'ok', got %q", view.ArchiveStatus)
		}
		if view.IsArchiving {
			t.Error("expected IsArchiving to be false for archived bookmark")
		}
	})

	t.Run("builds view for failed archive", func(t *testing.T) {
		id, err := server.db.AddBookmark("https://failed.com", "Failed")
		if err != nil {
			t.Fatalf("failed to add bookmark: %v", err)
		}
		now := time.Now()
		if err := server.db.SaveArchiveResult(id, now, nil, "error", "connection timeout", "", ""); err != nil {
			t.Fatalf("failed to save archive result: %v", err)
		}
		bookmark, err := server.db.GetBookmark(id)
		if err != nil {
			t.Fatalf("failed to get bookmark: %v", err)
		}

		view := server.buildArchiveManagerView(bookmark)

		if view.ArchiveStatus != "error" {
			t.Errorf("expected ArchiveStatus 'error', got %q", view.ArchiveStatus)
		}
		if view.ArchiveError != "connection timeout" {
			t.Errorf("expected ArchiveError 'connection timeout', got %q", view.ArchiveError)
		}
		if view.IsArchiving {
			t.Error("expected IsArchiving to be false for failed archive")
		}
	})
}

// itoa converts an int64 to string for URL building.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
