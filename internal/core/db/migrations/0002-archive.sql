-- Add fields for archiving/scraping bookmark pages

ALTER TABLE bookmarks ADD COLUMN archived_html TEXT;
ALTER TABLE bookmarks ADD COLUMN archived_url TEXT;
ALTER TABLE bookmarks ADD COLUMN archive_attempted_at TEXT;
ALTER TABLE bookmarks ADD COLUMN archived_at TEXT;
ALTER TABLE bookmarks ADD COLUMN archive_status TEXT;
ALTER TABLE bookmarks ADD COLUMN archive_error TEXT;

