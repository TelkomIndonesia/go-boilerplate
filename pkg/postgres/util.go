package postgres

import (
	"database/sql"
	"database/sql/driver"
	"hash/fnv"

	"github.com/lib/pq"
)

func txRollbackDeferer(tx *sql.Tx, err *error) func() {
	return func() {
		if *err != nil {
			tx.Rollback()
		}
	}
}

func pqByteArray(arr [][]byte) driver.Valuer {
	return pq.ByteaArray(arr)
}

func keyNameAsHash64(keyName string) uint64 {
	hash := fnv.New64()
	if _, err := hash.Write([]byte(keyName)); err != nil {
		panic(err)
	}
	return hash.Sum64()
}
