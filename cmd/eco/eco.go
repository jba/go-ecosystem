package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/jba/cli"
	"github.com/jba/go-ecosystem/ecodb"
	_ "modernc.org/sqlite"
)

var top = cli.Top(nil)

func main() {
	os.Exit(top.Main(context.Background()))
}

func openDB() *sql.DB {
	db, err := ecodb.Open()
	if err != nil {
		log.Fatalf("%s", err)
	}
	return db
}
