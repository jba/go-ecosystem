package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jba/go-ecosystem/index"
	"github.com/jba/go-ecosystem/internal/database"
	"github.com/jba/go-ecosystem/internal/progress"
	_ "modernc.org/sqlite"
)

func init() {
	top.Command("update", &updateCmd{}, "update the modules table from the index")
}

type updateCmd struct {
	Duration time.Duration
}

func (c *updateCmd) Run(ctx context.Context) error {
	db := openDB()
	defer db.Close()

	// Get the indexSince value from params table
	var since string
	err := db.QueryRowContext(ctx, "SELECT value FROM params WHERE name = 'indexSince'").Scan(&since)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("querying indexSince: %w", err)
	}
	log.Printf("reading index from %s", since)

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

	log.Printf("saw %d unique paths in index in %s", len(seen), c.Duration)
	// For each path not in modules table, add it with state "index"

	start := time.Now()
	have, err := allModulePaths(ctx, db)
	if err != nil {
		return err
	}
	log.Printf("read %d paths from DB in %s", len(have), time.Since(start))

	var toAdd []string
	for path := range seen {
		if !have[path] {
			toAdd = append(toAdd, path)
		}
	}
	log.Printf("adding %d paths", len(toAdd))

	p := progress.Start(len(toAdd), 10*time.Second, nil)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO modules (path, state) VALUES (?, 'index')")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, path := range toAdd {
		if _, err := stmt.ExecContext(ctx, path); err != nil {
			return fmt.Errorf("inserting module %s: %w", path, err)
		}
		p.Did(1)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("read index to %s", latestTimestamp)

	// Write the latest timestamp to params table
	if latestTimestamp != "" {
		_, err = db.ExecContext(ctx,
			"INSERT INTO params (name, value) VALUES ('indexSince', ?) ON CONFLICT(name) DO UPDATE SET value = ?",
			latestTimestamp, latestTimestamp)
		if err != nil {
			return fmt.Errorf("updating indexSince: %w", err)
		}
	}
	return nil
}

func allModulePaths(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	iter, errf := database.ScanRows[string](ctx, db, "SELECT path FROM modules")
	m := map[string]bool{}
	for p := range iter {
		m[p] = true
	}
	if err := errf(); err != nil {
		return nil, err
	}
	return m, nil
}
