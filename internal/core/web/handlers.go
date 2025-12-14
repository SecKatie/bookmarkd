package web

import (
	"log"
	"net/http"
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
