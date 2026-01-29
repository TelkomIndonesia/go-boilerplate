package types

import (
	"database/sql/driver"

	"github.com/jackc/pgx/v5/pgtype"
)

type Array[T any] pgtype.Array[T]

func (a Array[T]) Value() (driver.Value, error) {
	return pgtype.Array[T](a), nil
}

func NewArray[T any](arr []T) Array[T] {
	return Array[T]{
		Elements: arr,
		Valid:    true,
		Dims: []pgtype.ArrayDimension{{
			Length: int32(len(arr)),
		}},
	}
}

func NewArrayValuer[T any](arr []T) driver.Valuer {
	return NewArray(arr)
}
