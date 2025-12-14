/*
Copyright © 2025 Katie Mulliken <katie@mulliken.net>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/seckatie/bookmarkd/internal/core"
	"github.com/seckatie/bookmarkd/internal/core/db"
	"github.com/seckatie/bookmarkd/internal/core/web"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bookmarkd",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		database, err := initDB(cmd)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer func() {
			if err := database.Close(); err != nil {
				log.Printf("failed to close database: %v", err)
			}
		}()

		numWorkers, err := cmd.Flags().GetInt("archive-workers")
		if err != nil {
			log.Fatalf("Failed to get archive workers: %v", err)
		}

		// Create the work queue for the archive workers
		workQueue := make(chan db.Bookmark, numWorkers*10) // Buffer for multiple bookmarks

		// queueBookmark attempts to queue a bookmark for archiving.
		// It tries for up to 5 seconds before giving up. The bookmark will be
		// automatically retried on next startup since it remains unarchived in the DB.
		queueBookmark := func(bookmark db.Bookmark, reason string) {
			select {
			case workQueue <- bookmark:
				log.Printf("Queued bookmark %d (%s) for %s", bookmark.ID, bookmark.URL, reason)
			case <-time.After(5 * time.Second):
				log.Printf("Warning: work queue full after 5s, bookmark %d (%s) not queued for %s - will be retried on next startup",
					bookmark.ID, bookmark.URL, reason)
			}
		}

		// Register event listeners to queue bookmarks for archiving
		database.RegisterEventListener(db.OnBookmarkCreatedEvent, func(event db.Event) error {
			ev := event.(db.BookmarkCreatedEvent)
			queueBookmark(ev.Bookmark, "archiving (new)")
			return nil
		})

		database.RegisterEventListener(db.OnArchiveClearedEvent, func(event db.Event) error {
			ev := event.(db.ArchiveClearedEvent)
			log.Printf("Archive cleared for bookmark %d, queuing for re-archiving", ev.BookmarkID)
			// Fetch the bookmark to queue it
			bookmark, err := database.GetBookmark(ev.BookmarkID)
			if err != nil {
				log.Printf("Error fetching bookmark %d for re-archiving: %v", ev.BookmarkID, err)
				return err
			}
			queueBookmark(bookmark, "re-archiving")
			return nil
		})

		// Start archive workers that process bookmarks and persist results
		for i := 0; i < numWorkers; i++ {
			workerID := i
			go func() {
				log.Printf("Archive worker %d started", workerID)
				for bookmark := range workQueue {
					log.Printf("Worker %d archiving bookmark %d: %s", workerID, bookmark.ID, bookmark.URL)
					ctx := context.Background()
					if err := core.ArchiveAndPersist(ctx, database, bookmark, core.ArchiveOptions{
						Headless: true,
					}); err != nil {
						log.Printf("Worker %d: Archive failed for id=%d url=%s: %v", workerID, bookmark.ID, bookmark.URL, err)
					} else {
						log.Printf("Worker %d: Successfully archived bookmark %d", workerID, bookmark.ID)
					}
				}
				log.Printf("Archive worker %d stopped", workerID)
			}()
		}

		// On startup, check for any existing unarchived bookmarks and queue them
		go func() {
			time.Sleep(2 * time.Second) // Give the server a moment to start
			log.Println("Checking for existing unarchived bookmarks on startup...")
			bookmarks, err := database.ListBookmarksToArchive(0)
			if err != nil {
				log.Printf("Error listing bookmarks to archive: %v", err)
				return
			}
			if len(bookmarks) == 0 {
				log.Println("No existing bookmarks need archiving")
				return
			}
			log.Printf("Found %d existing unarchived bookmarks, queuing...", len(bookmarks))
			queued := 0
			for _, b := range bookmarks {
				select {
				case workQueue <- b:
					queued++
				case <-time.After(5 * time.Second):
					log.Printf("Warning: work queue full, stopped queuing at %d/%d bookmarks - remaining will be retried on next startup",
						queued, len(bookmarks))
					return
				}
			}
			log.Printf("Successfully queued all %d existing bookmarks for archiving", queued)
		}()

		// Get the host and port from the flags
		host, err := cmd.Flags().GetString("host")
		if err != nil {
			log.Fatalf("Failed to get host: %v", err)
		}
		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			log.Fatalf("Failed to get port: %v", err)
		}

		// Start the web server
		web.StartServer(fmt.Sprintf("%s:%d", host, port), database)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("db", "d", "bookmarkd.db", "Path to the SQLite database file")
	rootCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	rootCmd.Flags().String("host", "localhost", "Host to listen on")

	// Archive workers flags
	rootCmd.Flags().IntP("archive-workers", "w", 1, "Number of archive workers to run")
}

func initDB(cmd *cobra.Command) (*db.DB, error) {
	dbPath, err := cmd.Flags().GetString("db")
	if err != nil {
		log.Fatalf("Failed to get database path: %v", err)
	}
	database, err := db.NewSQLiteDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migrated successfully")

	return database, nil
}
