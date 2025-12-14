package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	db             *sql.DB
	eventListeners map[EventKind][]EventListener
}

func NewSQLiteDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &DB{
		db:             db,
		eventListeners: make(map[EventKind][]EventListener),
	}, nil
}

func (db *DB) Migrate() error {
	// Create migrations tracking table if it doesn't exist
	_, err := db.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migrations = append(migrations, entry.Name())
	}

	sort.Strings(migrations)

	for _, migration := range migrations {
		version := strings.TrimSuffix(migration, ".sql")
		if version == "" {
			log.Println("Invalid migration file name:", migration)
			continue
		}

		// Check if migration has already been applied
		var exists bool
		if err := db.db.QueryRow(`
		    SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)
		`, version).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check if migration has been applied: %w", err)
		}
		if exists {
			log.Printf("Migration %s has already been applied, skipping...", version)
			continue
		}

		// Apply migration
		content, err := migrationsFS.ReadFile("migrations/" + migration)
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}

		tx, err := db.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}

		// Mark migration as applied
		if _, err := tx.Exec(`
		    INSERT INTO schema_migrations (version) VALUES (?)
		`, version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to mark migration as applied: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		log.Printf("Migration %s applied successfully", version)
	}

	return nil
}

func (db *DB) Close() error {
	return db.db.Close()
}
