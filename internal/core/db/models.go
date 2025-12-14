package db

type Bookmark struct {
	ID    int64
	URL   string
	Title string
	// CreatedAt is stored in the DB as RFC3339 text.
	CreatedAt string
}

type BookmarkArchive struct {
	BookmarkID         int64
	ArchivedURL        string
	ArchivedHTML       string
	ArchiveAttemptedAt string
	ArchivedAt         string
	ArchiveStatus      string
	ArchiveError       string
}
