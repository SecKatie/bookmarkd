/*
Copyright © 2025 Katie Mulliken <katie@mulliken.net>
*/

// The archive command provides functionality to archive (scrape and store) bookmarks in the database.
//
// Features:
//   - Archive a single bookmark by specifying its ID.
//   - Archive multiple bookmarks by limiting the number processed.
//   - Customize the Chrome/Chromium executable path used for scraping.
//   - Choose between headless or headful Chrome execution.
//   - Configure a timeout for each archive job.
//   - Wait for a specified CSS selector before scraping, helpful for dynamic JS-rendered pages.
//
// Example usage:
//
//	bookmarkd archive --id=123 --limit=5 --timeout=30s --wait-selector=".loading-indicator" --chrome-path="/path/to/chrome" --headful
//	bookmarkd archive --limit=10 --headless
package cmd

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/seckatie/bookmarkd/internal/core"
	"github.com/spf13/cobra"
)

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive (scrape) bookmarks into the database",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runArchive(cmd); err != nil {
			log.Fatalf("Archive failed: %v", err)
		}
	},
}

// runArchive is the main function for the archive command.
func runArchive(cmd *cobra.Command) error {
	db, err := initDB(cmd)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	id, err := cmd.Flags().GetInt64("id")
	if err != nil {
		return fmt.Errorf("failed to read --id: %w", err)
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return fmt.Errorf("failed to read --limit: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return fmt.Errorf("failed to read --timeout: %w", err)
	}
	waitSelector, err := cmd.Flags().GetString("wait-selector")
	if err != nil {
		return fmt.Errorf("failed to read --wait-selector: %w", err)
	}
	chromePath, err := cmd.Flags().GetString("chrome-path")
	if err != nil {
		return fmt.Errorf("failed to read --chrome-path: %w", err)
	}
	headful, err := cmd.Flags().GetBool("headful")
	if err != nil {
		return fmt.Errorf("failed to read --headful: %w", err)
	}

	if chromePath == "" && runtime.GOOS == "darwin" {
		// Best-effort default for macOS.
		chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}

	opts := core.ArchiveOptions{
		ChromePath:   chromePath,
		Headless:     !headful,
		Timeout:      timeout,
		WaitSelector: waitSelector,
	}

	ctx := context.Background()

	if id > 0 {
		b, err := db.GetBookmark(id)
		if err != nil {
			return err
		}
		return core.ArchiveAndPersist(ctx, db, b, opts)
	}

	bookmarks, err := db.ListBookmarksToArchive(limit)
	if err != nil {
		return err
	}
	if len(bookmarks) == 0 {
		log.Println("No bookmarks to archive.")
		return nil
	}

	log.Printf("Archiving %d bookmark(s)...", len(bookmarks))
	var failures int
	for _, b := range bookmarks {
		if err := core.ArchiveAndPersist(ctx, db, b, opts); err != nil {
			failures++
			log.Printf("Archive failed for id=%d url=%s: %v", b.ID, b.URL, err)
		}
	}
	if failures > 0 {
		return fmt.Errorf("archiving finished with %d failure(s)", failures)
	}

	log.Println("Archiving finished successfully.")
	return nil
}

func init() {
	rootCmd.AddCommand(archiveCmd)

	archiveCmd.Flags().Int64("id", 0, "Archive a specific bookmark id")
	archiveCmd.Flags().Int("limit", 0, "Limit the number of bookmarks to archive (0 = all unarchived)")
	archiveCmd.Flags().Duration("timeout", 40*time.Second, "Per-bookmark archive timeout")
	archiveCmd.Flags().String("wait-selector", "", "Optional CSS selector to wait for (useful for JS-heavy pages)")
	archiveCmd.Flags().String("chrome-path", "", "Path to Chrome/Chromium executable")
	archiveCmd.Flags().Bool("headful", false, "Run Chrome with a visible window (not headless)")
}
