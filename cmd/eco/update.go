package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jba/go-ecosystem/index"
	_ "modernc.org/sqlite"
)

func init() {
	top.Command("update", &updateCmd{}, "update the modules table from the index")
}

type updateCmd struct {
	Duration time.Duration
}

func (c *updateCmd) Run(ctx context.Context) error {
	dir := os.Getenv("GOECODIR")
	if dir == "" {
		return fmt.Errorf("GOECODIR environment variable not set")
	}

	dbPath := filepath.Join(dir, "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Get the indexSince value from params table
	var since string
	err = db.QueryRowContext(ctx, "SELECT value FROM params WHERE name = 'indexSince'").Scan(&since)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("querying indexSince: %w", err)
	}

	// Call index.Entries
	entries, errFunc := index.Entries(ctx, since)

	// Collect paths and track the latest timestamp
	seen := make(map[string]bool)
	var latestTimestamp string
	deadline := time.Now().Add(c.Duration)

	for e := range entries {
		if time.Now().After(deadline) {
			break
		}
		seen[e.Path] = true
		latestTimestamp = e.Timestamp
	}
	if err := errFunc(); err != nil {
		return fmt.Errorf("reading index: %w", err)
	}

	// For each path not in modules table, add it with state "index"
	for path := range seen {
		var exists bool
		err := db.QueryRowContext(ctx, "SELECT 1 FROM modules WHERE path = ?", path).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("checking module %s: %w", path, err)
		}
		if err == sql.ErrNoRows {
			_, err = db.ExecContext(ctx, "INSERT INTO modules (path, state) VALUES (?, 'index')", path)
			if err != nil {
				return fmt.Errorf("inserting module %s: %w", path, err)
			}
		}
	}

	// Write the latest timestamp to params table
	if latestTimestamp != "" {
		_, err = db.ExecContext(ctx,
			"INSERT INTO params (name, value) VALUES ('indexSince', ?) ON CONFLICT(name) DO UPDATE SET value = ?",
			latestTimestamp, latestTimestamp)
		if err != nil {
			return fmt.Errorf("updating indexSince: %w", err)
		}
	}

	fmt.Printf("Processed %d unique module paths\n", len(seen))
	return nil
}
