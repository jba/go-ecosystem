package ecodb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// A Module is information about a Go module
// known to the proxy.
//
// If only Path is non-empty, the module has been seen in the index only.
// ID == 0 => not inserted.
type Module struct {
	ID            int64
	Path          string
	Error         string
	LatestVersion string
	InfoTime      string // from proxy info
}

var moduleCols = []string{"id", "path", "error", "latest_version", "info_time"}

var moduleSelectStmt = "SELECT " + cols(moduleCols) + " FROM modules"

func ScanModule(rows *sql.Rows) (*Module, error) {
	var m Module
	// order must match moduleColumns
	if err := rows.Scan(&m.ID, &m.Path, &m.Error, &m.LatestVersion, &m.InfoTime); err != nil {
		return nil, err
	}
	return &m, nil
}

var ModuleInsertStmt = "INSERT INTO modules " + cols(moduleCols[1:]) + " VALUES " + qmarks(len(moduleCols)-1)

var ModuleUpdateStmt = "UPDATE modules SET " + cols(moduleCols[2:]) + " = " + qmarks(len(moduleCols)-2) +
	" WHERE path = ?"

func (m *Module) InsertArgs() []any {
	return []any{m.Path, m.Error, m.LatestVersion, m.InfoTime}
}

func (m *Module) UpdateArgs() []any {
	return []any{m.Error, m.LatestVersion, m.InfoTime, m.Path}
}

func cols(cols []string) string {
	return "(" + strings.Join(cols, ", ") + ")"
}

func qmarks(n int) string {
	return "(" + strings.Repeat("?, ", n-1) + "?)"
}
