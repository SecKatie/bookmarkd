package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/seckatie/bookmarkd/internal/core/db"
)

//go:embed templates/*.html static/*.css
var templatesFS embed.FS

type Server struct {
	db                 *db.DB
	indexHTML          []byte
	bookmarksTmpl      *template.Template
	viewerTmpl         *template.Template
	archivesHTML       []byte
	archivesListTmpl   *template.Template
	archiveItemTmpl    *template.Template
	bookmarkletHTML    []byte
	bookmarkletAddTmpl *template.Template
	staticFS           http.FileSystem
}

func StartServer(addr string, database *db.DB) {
	ws, err := newServer(database)
	if err != nil {
		log.Fatalf("Failed to initialize web server: %v", err)
	}

	mux := http.NewServeMux()
	ws.registerRoutes(mux)

	log.Printf("Starting web server at %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}

func newServer(database *db.DB) (*Server, error) {
	indexHTML, err := templatesFS.ReadFile("templates/index.html")
	if err != nil {
		return nil, err
	}

	bookmarksTmpl, err := template.ParseFS(templatesFS, "templates/bookmarks.html")
	if err != nil {
		return nil, err
	}

	viewerTmpl, err := template.ParseFS(templatesFS, "templates/viewer.html")
	if err != nil {
		return nil, err
	}

	archivesHTML, err := templatesFS.ReadFile("templates/archives.html")
	if err != nil {
		return nil, err
	}

	archivesListTmpl, err := template.ParseFS(templatesFS, "templates/archives_list.html")
	if err != nil {
		return nil, err
	}

	archiveItemTmpl, err := template.ParseFS(templatesFS, "templates/archive_item.html")
	if err != nil {
		return nil, err
	}

	bookmarkletHTML, err := templatesFS.ReadFile("templates/bookmarklet.html")
	if err != nil {
		return nil, err
	}

	bookmarkletAddTmpl, err := template.ParseFS(templatesFS, "templates/bookmarklet_add.html")
	if err != nil {
		return nil, err
	}

	staticSub, err := fs.Sub(templatesFS, "static")
	if err != nil {
		return nil, err
	}

	return &Server{
		db:                 database,
		indexHTML:          indexHTML,
		bookmarksTmpl:      bookmarksTmpl,
		viewerTmpl:         viewerTmpl,
		archivesHTML:       archivesHTML,
		archivesListTmpl:   archivesListTmpl,
		archiveItemTmpl:    archiveItemTmpl,
		bookmarkletHTML:    bookmarkletHTML,
		bookmarkletAddTmpl: bookmarkletAddTmpl,
		staticFS:           http.FS(staticSub),
	}, nil
}

func (ws *Server) registerRoutes(mux *http.ServeMux) {
	ws.registerStaticRoutes(mux)

	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/bookmarklet/add", ws.handleBookmarkletAdd)
	mux.HandleFunc("/bookmarklet", ws.handleBookmarklet)
	mux.HandleFunc("/bookmarks", ws.handleBookmarks)
	mux.HandleFunc("/bookmarks/", ws.handleArchive) // Handles /bookmarks/{id}/archive and /bookmarks/{id}/archive/raw
	mux.HandleFunc("/archives", ws.handleArchiveManager)
	mux.HandleFunc("/archives/", ws.handleArchivesRoutes) // Handles /archives/list and /archives/{id}/refetch
}

func (ws *Server) registerStaticRoutes(mux *http.ServeMux) {
	// Serve embedded static assets (CSS, etc)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(ws.staticFS)))
}
