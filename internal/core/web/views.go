package web

type bookmarkView struct {
	ID            int64
	URL           string
	Title         string
	ArchiveStatus string // "", "ok", "error"
	ArchivedAt    string
}

type archiveView struct {
	ID     int64
	URL    string
	Title  string
	RawURL string // URL to fetch raw archived HTML
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
