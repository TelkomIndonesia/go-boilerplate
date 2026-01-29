package types

import (
	"database/sql/driver"

	"github.com/jackc/pgx/v5/pgtype"
)

type ByteaArray [][]byte

func NewByteArray(arr [][]byte) driver.Valuer {
	return ByteaArray(arr)
}

func (a ByteaArray) Value() (driver.Value, error) {
	arr := pgtype.Array[[]byte]{
		Elements: [][]byte(a),
		Valid:    true,
		Dims: []pgtype.ArrayDimension{{
			Length: int32(len(a)),
		}},
	}
	return arr, nil
}
