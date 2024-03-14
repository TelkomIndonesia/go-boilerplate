package postgres

import (
	"database/sql"
	"hash/fnv"
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

func keyNameAsHash64(keyName string) uint64 {
	hash := fnv.New64()
	_, err := hash.Write([]byte(keyName))
	if err != nil {
		panic(err)
	}
	return hash.Sum64()
}
