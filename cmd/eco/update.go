package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
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
	Module   string `cli:"flag=mod"`
}

func (c *updateCmd) Run(ctx context.Context) error {
	if c.Module != "" {
		m := &ecodb.Module{Path: c.Module}
		if err := populateModuleFromProxy(ctx, m); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%+v\n", m)
		return nil
	}

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
	if err := c.updateModuleFromProxy(ctx, db, mods); err != nil {
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
			mod, inDB := mods[p]
			// If the mod is in the DB, this will effectively clear out all other columns.
			if inDB {
				// This path is in the DB, but since we saw it again in the index, redo everything.
				mod = &ecodb.Module{ID: mod.ID, Path: mod.Path}
				nUpdates++
				if _, err := update.ExecContext(ctx, mod.UpdateArgs()...); err != nil {
					return err
				}
			} else {
				mod = &ecodb.Module{Path: p}
				mods[p] = mod
				nInserts++
				res, err := insert.ExecContext(ctx, mod.InsertArgs()...)
				if err != nil {
					return err
				}
				mod.ID, err = res.LastInsertId()
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("%d inserts and %d updates in %.1fs", nInserts, nUpdates, time.Since(start).Seconds())

	// Write the latest timestamp to params table.
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

func (c *updateCmd) updateModuleFromProxy(ctx context.Context, db *sql.DB, mods map[string]*ecodb.Module) error {
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

	proxy.SetMaxQPS(300)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	// sqlite can only do one write at a time
	var mu sync.Mutex

	var proxyDur, dbDur atomic.Int64

	for _, mod := range toUpdate {
		g.Go(func() error {
			start := time.Now()
			if err := populateModuleFromProxy(gctx, mod); err != nil {
				return err
			}
			proxyDur.Add(time.Since(start).Nanoseconds())
			start = time.Now()
			mu.Lock()
			if _, err := db.ExecContext(gctx, ecodb.ModuleUpdateStmt, mod.UpdateArgs()...); err != nil {
				mu.Unlock()
				return err
			}
			mu.Unlock()
			dbDur.Add(time.Since(start).Nanoseconds())
			p.Did(1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	log.Printf("proxy: %.1fs, db: %.1fs", time.Duration(proxyDur.Load()).Seconds(),
		time.Duration(dbDur.Load()).Seconds())
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
