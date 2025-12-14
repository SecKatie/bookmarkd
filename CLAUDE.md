# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

bookmarkd is a self-hosted bookmark archiving service written in Go. It saves web pages locally by capturing rendered HTML (including JS-heavy SPAs) via Chrome/Chromium headless browser and inlining external resources (CSS, JS, images) to create self-contained offline archives.

## Common Commands

```bash
# Run the server (starts web UI + background archive workers)
go run . --port 8080 --host localhost --db bookmarkd.db --archive-workers 2

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/core/db/...
go test ./cmd/...

# Run a single test
go test ./internal/core/db/... -run TestAddBookmark

# Archive bookmarks via CLI (instead of background workers)
go run . archive --limit=10 --headless
go run . archive --id=123 --timeout=30s

# Build
go build -o bookmarkd .
```

## Architecture

### Package Structure

- `cmd/` - Cobra CLI commands (root server command, archive subcommand)
- `internal/core/` - Core business logic
  - `archive.go` - Browser-based page capture using chromedp
  - `inline.go` - Resource inlining (CSS, JS, images → data URIs)
  - `db/` - SQLite database layer with embedded migrations
    - `events.go` - Event system for bookmark/archive lifecycle hooks
    - `bookmarks.go`, `archives.go` - Data access methods
    - `migrations/*.sql` - Embedded SQL migrations (auto-applied)
  - `web/` - HTTP server with embedded templates
    - `handlers.go` - Request handlers
    - `templates/*.html` - HTML templates
    - `static/*.css` - Static assets

### Key Patterns

**Event-Driven Archiving**: The database emits events (`OnBookmarkCreatedEvent`, `OnArchiveClearedEvent`) that trigger background archive workers. Register listeners via `db.RegisterEventListener()`.

**Embedded Assets**: Templates, static files, and migrations are embedded via `//go:embed`. Changes to these files require rebuild.

**Archive Pipeline**: `ArchiveBookmark()` → chromedp captures rendered HTML → `InlineResources()` converts external resources to data URIs → `SaveArchiveResult()` persists to SQLite.

### Web Routes

- `/` - Bookmark list (main UI)
- `/bookmarks` - POST to add, GET to list
- `/bookmarklet` - Bookmarklet installation page
- `/bookmarklet/add` - Bookmarklet endpoint
- `/bookmarks/{id}/archive` - View archived page
- `/bookmarks/{id}/archive/raw` - Raw archived HTML
- `/archives` - Archive management UI
- `/archives/{id}/refetch` - Re-queue bookmark for archiving

## Testing

Tests use the standard Go testing package. Database tests create temporary SQLite files. Web handler tests use `httptest.NewRecorder()`.
