package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jba/cli"
	_ "modernc.org/sqlite"
)

var top = cli.Top(nil)

func main() {
	os.Exit(top.Main(context.Background()))
}

func init() {
	top.Command("create-db", &createDBCmd{}, "create the database")
}

type createDBCmd struct{}

func (c *createDBCmd) Run(ctx context.Context) error {
	dir := os.Getenv("GOECODIR")
	if dir == "" {
		return fmt.Errorf("GOECODIR environment variable not set")
	}

	dbPath := filepath.Join(dir, "db.sqlite")

	// Read db.sql
	sqlBytes, err := os.ReadFile("db.sql")
	if err != nil {
		return fmt.Errorf("reading db.sql: %w", err)
	}

	// Create and open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Execute SQL to create tables
	if _, err := db.Exec(string(sqlBytes)); err != nil {
		return fmt.Errorf("executing db.sql: %w", err)
	}

	log.Printf("Created database at %s", dbPath)
	return nil
}
