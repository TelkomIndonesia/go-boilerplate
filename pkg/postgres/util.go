package postgres

import (
	"database/sql"
)

func txRollbackDeferer(tx *sql.Tx, err *error) func() {
	return func() {
		if *err != nil {
			tx.Rollback()
		}
	}
}

func min[T int | int16 | int32 | int64](a T, b T) T {
	if a < b {
		return a
	}
	return b
}
