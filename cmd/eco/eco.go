package main

import (
	"context"
	"database/sql"
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

func openDB() *sql.DB {
	dir := os.Getenv("GOECODIR")
	if dir == "" {
		log.Fatal("GOECODIR environment variable not set")
	}

	dbPath := filepath.Join(dir, "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("opening database %s: %w", dbPath, err)
	}
	log.Printf("opened DB at %s", dbPath)
	return db
}
