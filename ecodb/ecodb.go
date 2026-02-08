package ecodb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	InfoTime      string // from proxy info
	Origin        string // JSON of proxy.Origin
	Error         string
}

func ScanModule(rows *sql.Rows) (*Module, error) {
	var m Module
	var lv, er, it, or sql.NullString
	if err := rows.Scan(&m.ID, &m.Path, &m.State, &lv, &er, &it, &or); err != nil {
		return nil, err
	}
	if lv.Valid {
		m.LatestVersion = lv.String
	}
	if er.Valid {
		m.Error = er.String
	}
	if it.Valid {
		m.InfoTime = it.String
	}
	if or.Valid {
		m.Origin = or.String
	}
	return &m, nil
}
