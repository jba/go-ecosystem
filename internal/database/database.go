package database

import (
	"context"
	"database/sql"
	"iter"

	"github.com/jba/go-ecosystem/internal/jiter"
)

func ScanRows(ctx context.Context, db *sql.DB, query string, params ...any) (iter.Seq[*sql.Rows], func() error) {
	var es jiter.ErrorState
	return func(yield func(*sql.Rows) bool) {
		rows, err := db.QueryContext(ctx, query, params...)
		if err != nil {
			es.Set(err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			if !yield(rows) {
				return
			}
		}
		es.Set(rows.Err())
	}, es.Func()
}

func ScanRowsOf[T any](ctx context.Context, db *sql.DB, query string, params ...any) (iter.Seq[T], func() error) {
	var es jiter.ErrorState
	return func(yield func(T) bool) {
		iter, errf := ScanRows(ctx, db, query, params...)
		for rows := range iter {
			var x T
			if err := rows.Scan(&x); err != nil {
				es.Set(err)
				return
			}
			if !yield(x) {
				return
			}
		}
		es.Set(errf())
	}, es.Func()
}

func Transaction(db *sql.DB, f func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := f(tx); err != nil {
		return err
	}
	return tx.Commit()
}
