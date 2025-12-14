package web

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// renderTemplate renders a template with the standard HTML content-type header.
// If template execution fails, it logs the error and returns a 500 response.
func (ws *Server) renderTemplate(w http.ResponseWriter, templateName string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ws.templates.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to execute %s template: %v", templateName, err)
	}
}

// requireMethod checks if the request method matches the expected method.
// Returns true if the method matches, false otherwise (and sends 405 response).
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// parseIDFromPath extracts an int64 ID from a URL path segment.
// For path "/bookmarks/123/archive", prefix "/bookmarks/", returns 123.
func parseIDFromPath(path, prefix string) (int64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, errors.New("missing ID in path")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
