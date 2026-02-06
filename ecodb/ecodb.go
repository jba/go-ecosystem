package ecodb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Open() (*sql.DB, error) {
	dir := os.Getenv("GOECODIR")
	if dir == "" {
		return nil, errors.New("ecodb.Open: GOECODIR environment variable not set")
	}

	dbPath := filepath.Join(dir, "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	return db, nil
}

type Module struct {
	ID            int64
	Path          string
	State         string
	LatestVersion string
	InfoTime      time.Time // from proxy info
	Error         string
}

func scanModule(rows *sql.Rows) (*Module, error) {
	var m Module
	if err := rows.Scan(&m.ID, &m.Path, &m.State, &m.LatestVersion, &m.Error, &m.InfoTime); err != nil {
		return nil, err
	}
	return &m, nil
}
