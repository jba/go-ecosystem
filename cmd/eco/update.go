package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jba/go-ecosystem/ecodb"
	"github.com/jba/go-ecosystem/index"
	"github.com/jba/go-ecosystem/internal/database"
	"github.com/jba/go-ecosystem/internal/progress"
	"github.com/jba/go-ecosystem/proxy"
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

	if err := c.updateFromIndex(ctx, db); err != nil {
		return err
	}
	// if err := c.updateLatestVersions(ctx, db); err != nil {
	// 	return err
	// }
	return nil
}

func reportProgressWithProxy(i progress.Info) {
	var qs string
	if q := proxy.QPS(); q > 0 {
		qs = fmt.Sprintf(", proxy QPS = %.1f", q)
	}
	log.Printf("%s%s", i, qs)
}

func (c *updateCmd) updateFromIndex(ctx context.Context, db *sql.DB) error {
	// Read all modules into memory.
	start := time.Now()
	mods, err := allModules(ctx, db)
	if err != nil {
		return err
	}
	log.Printf("read %d modules from DB in %.1fs", len(mods), time.Since(start).Seconds())

	// Get the indexSince value from params table.
	var since string
	err = db.QueryRowContext(ctx, "SELECT value FROM params WHERE name = 'indexSince'").Scan(&since)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("querying indexSince: %w", err)
	}

	// Read the index.
	log.Printf("reading index from %s", since)

	// Collect paths and track the latest timestamp
	seen := make(map[string]bool)
	var latestTimestamp string
	deadline := time.Now().Add(c.Duration)

	entries, errf := index.Entries(ctx, since)
	for e := range entries {
		if time.Now().After(deadline) {
			break
		}
		seen[e.Path] = true
		latestTimestamp = e.Timestamp
	}
	if err := errf(); err != nil {
		return fmt.Errorf("reading index: %w", err)
	}

	log.Printf("saw %d unique paths in index in %s", len(seen), c.Duration)

	var inserts, updates []*ecodb.Module
	p := progress.Start(len(seen), 10*time.Second, reportProgressWithProxy)
	for path := range seen {
		mod := mods[path]
		latestVersion, err := latestModuleVersion(ctx, path)
		if err != nil {
			if errors.Is(err, errNoVersions) {
				if mod == nil {
					inserts = append(inserts, &ecodb.Module{Path: path, Error: err.Error()})
				} else {
					mod.Error = err.Error()
					updates = append(updates, mod)
				}
			} else {
				return err
			}
		} else if mod != nil && mod.LatestVersion == latestVersion && mod.InfoTime != "" {
			// The InfoTime check is temporary, while the DB has rows inserted before this logic.
			// do nothing
		} else {
			// Get info for this version.
			info, err := proxy.Info(ctx, path, latestVersion)
			if err != nil {
				return err
			}
			// insert/update all info
			if mod == nil {
				mod = &ecodb.Module{Path: path}
				inserts = append(inserts, mod)
			} else {
				updates = append(updates, mod)
			}
			mod.LatestVersion = latestVersion
			mod.InfoTime = info.Time
			origin, err := json.Marshal(info.Origin)
			if err != nil {
				return fmt.Errorf("marshaling origin: %w", err)
			}
			mod.Origin = string(origin)
		}
		p.Did(1)
	}
	log.Printf("inserting %d modules, updating %d", len(inserts), len(updates))

	p = progress.Start(len(inserts)+len(updates), 10*time.Second, nil)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO modules
		(path, latest_version, info_time, origin)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, mod := range inserts {
		if _, err := stmt.ExecContext(ctx, mod.Path, mod.LatestVersion, mod.InfoTime, mod.Origin); err != nil {
			return fmt.Errorf("inserting module %s: %w", mod.Path, err)
		}
		p.Did(1)
	}

	noErrorStmt, err := tx.PrepareContext(ctx, `
		UPDATE modules
		SET (latest_version, info_time, origin, error) = (?, ?, ?, NULL)
		WHERE path = ?`)
	if err != nil {
		return err
	}
	errorStmt, err := tx.PrepareContext(ctx, `
		UPDATE modules
		SET (latest_version, info_time, origin, error) = (NULL, NULL, NULL, ?)
		WHERE path = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, mod := range updates {
		var err error
		if mod.Error == "" {
			_, err = noErrorStmt.ExecContext(ctx, mod.LatestVersion, mod.InfoTime, mod.Origin, mod.Path)
		} else {
			_, err = errorStmt.ExecContext(ctx, mod.Error, mod.Path)
		}
		if err != nil {
			return err
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

func allModules(ctx context.Context, db *sql.DB) (map[string]*ecodb.Module, error) {
	iter, errf := database.ScanRows(ctx, db, "SELECT * FROM modules")
	mods := map[string]*ecodb.Module{}
	for r := range iter {
		m, err := ecodb.ScanModule(r)
		if err != nil {
			return nil, err
		}
		mods[m.Path] = m
	}
	if err := errf(); err != nil {
		return nil, err
	}
	return mods, nil
}

func (c *updateCmd) updateLatestVersions(ctx context.Context, db *sql.DB) error {
	log.Printf("ulv")
	return nil
}
