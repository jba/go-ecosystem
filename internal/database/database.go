package database

import (
	"context"
	"database/sql"
	"iter"

	"github.com/jba/go-ecosystem/internal/jiter"
)

func ScanRows[T any](ctx context.Context, db *sql.DB, query string, params ...any) (iter.Seq[T], func() error) {
	var es jiter.ErrorState
	return func(yield func(T) bool) {
		rows, err := db.QueryContext(ctx, query, params...)
		if err != nil {
			es.Set(err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var x T
			if err := rows.Scan(&x); err != nil {
				es.Set(err)
				return
			}
			if !yield(x) {
				return
			}
		}
		es.Set(rows.Err())
	}, es.Func()
}
