package types

import "database/sql"

// NullInt64 maps JSON-optional fields (*int) to nullable integer columns.
type NullInt64 struct {
	sql.NullInt64
}

// NewNullInt64 builds a NullInt64; nil pointers become NULL.
func NewNullInt64[T int | *int | int64 | *int64](n T) NullInt64 {
	switch v := any(n).(type) {
	case int:
		return NullInt64{Int64: int64(v), Valid: true}
	case int64:
		return NullInt64{Int64: v, Valid: true}
	case *int:
		if v == nil {
			return NullInt64{}
		}
		return NullInt64{Int64: int64(*v), Valid: true}
	case *int64:
		if v == nil {
			return NullInt64{}
		}
		return NullInt64{Int64: *v, Valid: true}
	default:
		panic("NewNullInt64: unsupported type")
	}
}
