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

func keyNameAsHash64(keyName string) int64 {
	hash := fnv.New64()
	if _, err := hash.Write([]byte(keyName)); err != nil {
		panic(err)
	}
	return int64(hash.Sum64())
}
