package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/jba/go-ecosystem/ecodb"
	"github.com/jba/go-ecosystem/index"
	"github.com/jba/go-ecosystem/internal/database"
	"github.com/jba/go-ecosystem/internal/httputil"
	"github.com/jba/go-ecosystem/internal/progress"
	"github.com/jba/go-ecosystem/proxy"
	"golang.org/x/sync/errgroup"
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

	// Read all modules into memory.
	start := time.Now()
	mods, err := allModules(ctx, db)
	if err != nil {
		return err
	}
	log.Printf("read %d modules from DB in %.1fs", len(mods), time.Since(start).Seconds())

	if err := c.updateFromIndex(ctx, db, mods); err != nil {
		return err
	}
	if err := c.updateFromProxy(ctx, db, mods); err != nil {
		return err
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

func (c *updateCmd) updateFromIndex(ctx context.Context, db *sql.DB, mods map[string]*ecodb.Module) error {
	// Get the indexSince value from params table.
	var since string
	err := db.QueryRowContext(ctx, "SELECT value FROM params WHERE name = 'indexSince'").Scan(&since)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("querying indexSince: %w", err)
	}

	// Read the index.
	log.Printf("reading index from %s", since)

	// Collect unique paths and track the latest timestamp
	seen := map[string]bool{}
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

	// Write the new modules.
	nInserts := 0
	nUpdates := 0
	start := time.Now()
	err = database.Transaction(db, func(tx *sql.Tx) error {
		insert, err := tx.PrepareContext(ctx, ecodb.ModuleInsertStmt)
		if err != nil {
			return err
		}
		defer insert.Close()
		update, err := tx.PrepareContext(ctx, ecodb.ModuleUpdateStmt)
		if err != nil {
			return err
		}
		defer update.Close()

		for p := range seen {
			// NOTE: this loses the IDs of all existing mods in seen.
			_, inDB := mods[p]
			// If the mod is in the DB, this will effectively clear out all other columns.
			m := &ecodb.Module{Path: p}
			mods[p] = m
			var err error
			if inDB {
				// This path is in the DB, but since we saw it again in the index, redo everything.
				nUpdates++
				_, err = update.ExecContext(ctx, m.UpdateArgs()...)
			} else {
				nInserts++
				_, err = insert.ExecContext(ctx, m.InsertArgs()...)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("%d inserts and %d updates in %.1fs", nInserts, nUpdates, time.Since(start).Seconds())

	// Write the latest timestamp to params table
	if latestTimestamp != "" {
		_, err = db.ExecContext(ctx,
			"INSERT INTO params (name, value) VALUES ('indexSince', ?) ON CONFLICT(name) DO UPDATE SET value = ?",
			latestTimestamp, latestTimestamp)
		if err != nil {
			return fmt.Errorf("updating indexSince: %w", err)
		}
	}
	log.Printf("read index to %s", latestTimestamp)
	return nil
}

func (c *updateCmd) updateFromProxy(ctx context.Context, db *sql.DB, mods map[string]*ecodb.Module) error {
	// Collect the modules that need information from the proxy.
	// We collect first so we can report accurate progress.
	var toUpdate []*ecodb.Module
	for _, m := range mods {
		if m.Error == "" && (m.LatestVersion == "" || m.InfoTime == "") {
			toUpdate = append(toUpdate, m)
		}
	}
	log.Printf("%d modules to update", len(toUpdate))
	p := progress.Start(len(toUpdate), 10*time.Second, reportProgressWithProxy)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for chunk := range slices.Chunk(toUpdate, 100) {
		g.Go(func() error {
			for _, mod := range chunk {
				if err := populateModuleFromProxy(gctx, mod); err != nil {
					return err
				}
				p.Did(1)
			}
			return nil
		})
		// Update the DB for this chunk.
		log.Printf("updating DB with %d changes", len(chunk))
		err := database.Transaction(db, func(tx *sql.Tx) error {
			update, err := tx.PrepareContext(ctx, ecodb.ModuleUpdateStmt)
			if err != nil {
				return err
			}
			defer update.Close()
			for _, mod := range chunk {
				if _, err := update.ExecContext(ctx, mod.UpdateArgs()...); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		log.Printf("updated DB")
	}
	return nil
}

func populateModuleFromProxy(ctx context.Context, mod *ecodb.Module) error {
	if mod.LatestVersion == "" {
		latestVersion, err := latestModuleVersion(ctx, mod.Path)
		if err != nil {
			if errors.Is(err, errNoVersions) || isNotFound(err) {
				mod.Error = err.Error()
			} else {
				return err
			}
		} else {
			mod.LatestVersion = latestVersion
		}
	}
	if mod.LatestVersion != "" {
		info, err := proxy.Info(ctx, mod.Path, mod.LatestVersion)
		if err != nil {
			return err
		}
		mod.InfoTime = info.Time
	}
	return nil
}

func isNotFound(err error) bool {
	s := httputil.ErrorStatus(err)
	return s == http.StatusNotFound || s == http.StatusGone
}

// 	for path := range seen {
// 		mod := mods[path]
// 		latestVersion, err := latestModuleVersion(ctx, path)
// 		if err != nil {
// 			if errors.Is(err, errNoVersions) {
// 				if mod == nil {
// 					inserts = append(inserts, &ecodb.Module{Path: path, Error: err.Error()})
// 				} else {
// 					mod.Error = err.Error()
// 					updates = append(updates, mod)
// 				}
// 			} else {
// 				return err
// 			}
// 		} else if mod != nil && mod.LatestVersion == latestVersion && mod.InfoTime != "" {
// 			// The InfoTime check is temporary, while the DB has rows inserted before this logic.
// 			// do nothing
// 		} else {
// 			// Get info for this version.
// 			info, err := proxy.Info(ctx, path, latestVersion)
// 			if err != nil {
// 				return err
// 			}
// 			// insert/update all info
// 			if mod == nil {
// 				mod = &ecodb.Module{Path: path}
// 				inserts = append(inserts, mod)
// 			} else {
// 				updates = append(updates, mod)
// 			}
// 			mod.LatestVersion = latestVersion
// 			mod.InfoTime = info.Time
// 		}
// 		p.Did(1)
// 	}
// 	log.Printf("inserting %d modules, updating %d", len(inserts), len(updates))

// 	p = progress.Start(len(inserts)+len(updates), 10*time.Second, nil)
// 	tx, err := db.Begin()
// 	if err != nil {
// 		return err
// 	}
// 	defer tx.Rollback()
// 	stmt, err := tx.PrepareContext(ctx, `
// 		INSERT INTO modules
// 		(path, latest_version, info_time, origin)
// 		VALUES (?, ?, ?, ?)`)
// 	if err != nil {
// 		return err
// 	}
// 	defer stmt.Close()
// 	for _, mod := range inserts {
// 		if _, err := stmt.ExecContext(ctx, mod.Path, mod.LatestVersion, mod.InfoTime); err != nil {
// 			return fmt.Errorf("inserting module %s: %w", mod.Path, err)
// 		}
// 		p.Did(1)
// 	}

// 	noErrorStmt, err := tx.PrepareContext(ctx, `
// 		UPDATE modules
// 		SET (latest_version, info_time, origin, error) = (?, ?, ?, NULL)
// 		WHERE path = ?`)
// 	if err != nil {
// 		return err
// 	}
// 	errorStmt, err := tx.PrepareContext(ctx, `
// 		UPDATE modules
// 		SET (latest_version, info_time, origin, error) = (NULL, NULL, NULL, ?)
// 		WHERE path = ?`)
// 	if err != nil {
// 		return err
// 	}
// 	defer stmt.Close()
// 	for _, mod := range updates {
// 		var err error
// 		if mod.Error == "" {
// 			_, err = noErrorStmt.ExecContext(ctx, mod.LatestVersion, mod.InfoTime, mod.Path)
// 		} else {
// 			_, err = errorStmt.ExecContext(ctx, mod.Error, mod.Path)
// 		}
// 		if err != nil {
// 			return err
// 		}
// 		p.Did(1)
// 	}

// 	if err := tx.Commit(); err != nil {
// 		return err
// 	}
// 	log.Printf("read index to %s", latestTimestamp)

// 	return nil
// }

func reportProgressWithProxy(i progress.Info) {
	var qs string
	if q := proxy.QPS(); q > 0 {
		qs = fmt.Sprintf(", proxy QPS = %.1f", q)
	}
	log.Printf("%s%s", i, qs)
}

func (c *updateCmd) updateLatestVersions(ctx context.Context, db *sql.DB) error {
	log.Printf("ulv")
	return nil
}
