package database

import (
	"context"
	"database/sql"
	"iter"
)

// ScanRows runs the query and returns an iterator over the resulting rows.
// If an error occurs, the iterator yields a nil *sql.Rows with a non-nil
// error and stops.
func ScanRows(ctx context.Context, db *sql.DB, query string, params ...any) iter.Seq2[*sql.Rows, error] {
	return func(yield func(*sql.Rows, error) bool) {
		rows, err := db.QueryContext(ctx, query, params...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			if !yield(rows, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// ScanRowsOf runs the query and returns an iterator over rows scanned into
// values of type T. If an error occurs, the iterator yields the zero value of
// T with a non-nil error and stops.
func ScanRowsOf[T any](ctx context.Context, db *sql.DB, query string, params ...any) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for rows, err := range ScanRows(ctx, db, query, params...) {
			var x T
			if err != nil {
				yield(x, err)
				return
			}
			if err := rows.Scan(&x); err != nil {
				yield(x, err)
				return
			}
			if !yield(x, nil) {
				return
			}
		}
	}
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
