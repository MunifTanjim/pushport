package types

import (
	"database/sql"
)

// NullString maps JSON-optional fields (*string) to nullable columns.
type NullString struct {
	sql.NullString
}

func NewNullString[T string | *string](s T) NullString {
	if p, ok := any(s).(*string); ok {
		if p == nil {
			return NullString{}
		}
		return NullString{sql.NullString{String: *p, Valid: true}}
	}
	return NullString{sql.NullString{String: any(s).(string), Valid: true}}
}

// ToStrPtr returns the value as *string: NULL becomes nil.
func (n NullString) ToStrPtr() *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}
