package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func init() {
	top.Command("create-db", &createDBCmd{}, "create the database")
}

type createDBCmd struct{}

func (c *createDBCmd) Run(ctx context.Context) error {
	// Read db.sql
	sqlBytes, err := os.ReadFile("db.sql")
	if err != nil {
		return fmt.Errorf("reading db.sql: %w", err)
	}

	// Create and open database
	db := openDB()
	defer db.Close()

	// Execute SQL to create tables
	if _, err := db.Exec(string(sqlBytes)); err != nil {
		return fmt.Errorf("executing db.sql: %w", err)
	}

	return nil
}
