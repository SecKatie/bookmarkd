package web

import (
	"log"
	"net/http"
)

func (ws *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ws.renderTemplate(w, "index.html", map[string]any{"ActivePage": "bookmarks"})
}

func (ws *Server) handleBookmarklet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ws.renderTemplate(w, "bookmarklet.html", map[string]any{"ActivePage": "bookmarklet"})
}

func (ws *Server) handleBookmarkletAdd(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	url := r.URL.Query().Get("url")
	title := r.URL.Query().Get("title")

	if url == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	if title == "" {
		title = url // Fallback to URL if title is empty
	}

	ws.renderTemplate(w, "bookmarklet_add.html", map[string]string{
		"URL":   url,
		"Title": title,
	})
}

func (ws *Server) handleBookmarks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ws.createBookmark(w, r)
		return
	case http.MethodGet:
		ws.listBookmarks(w, r)
		return
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (ws *Server) createBookmark(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	title := r.FormValue("title")

	if _, err := ws.db.AddBookmark(url, title); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to insert bookmark: %v", err)
		return
	}

	// For HTMX requests, return the updated list fragment directly so the page can swap
	// cleanly without a redirect.
	if r.Header.Get("HX-Request") == "true" {
		ws.listBookmarks(w, r)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ws *Server) listBookmarks(w http.ResponseWriter, _ *http.Request) {
	bookmarks, err := ws.db.ListBookmarks(0)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to get bookmarks: %v", err)
		return
	}

	var bookmarksData []bookmarkView
	for _, b := range bookmarks {
		view := bookmarkView{
			ID:    b.ID,
			URL:   b.URL,
			Title: b.Title,
		}
		// Fetch archive status for this bookmark
		archive, err := ws.db.GetBookmarkArchive(b.ID)
		if err == nil {
			view.ArchiveStatus = archive.ArchiveStatus
			view.ArchivedAt = archive.ArchivedAt
		}
		bookmarksData = append(bookmarksData, view)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ws.templates.ExecuteTemplate(w, "bookmarks.html", map[string]any{"bookmarks": bookmarksData}); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute template: %v", err)
		return
	}
}
