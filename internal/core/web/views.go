package web

type bookmarkView struct {
	ID            int64
	URL           string
	Title         string
	ArchiveStatus string // "", "ok", "error"
	ArchivedAt    string
}

type archiveManagerView struct {
	ID                 int64
	URL                string
	Title              string
	ArchiveStatus      string // "", "ok", "error"
	ArchivedAt         string
	ArchiveAttemptedAt string
	ArchiveError       string
	IsArchiving        bool // true when archive is queued or in progress
}
