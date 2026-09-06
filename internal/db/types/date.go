// Package types holds custom column types shared by the generated sqlc code and
// the db package. It must stay a leaf (stdlib-only) so the generated package can
// import it without an import cycle with internal/db.
package types

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DateLayout is lexicographically sortable, so SQLite range comparisons on a
// stored Date behave like plain string dates.
const DateLayout = "2006-01-02"

// Date is a calendar date (no time-of-day) backed by a DATE column, embedding
// time.Time but serializing via the fixed DateLayout.
type Date struct {
	time.Time
}

// NewDate returns the Date for t's UTC calendar day (time-of-day discarded).
func NewDate(t time.Time) Date {
	y, m, d := t.UTC().Date()
	return Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, err
	}
	return Date{Time: t}, nil
}

func (d Date) String() string { return d.UTC().Format(DateLayout) }

func (d Date) Value() (driver.Value, error) { return d.String(), nil }

func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		d.Time = time.Time{}
	case string:
		t, err := time.Parse(DateLayout, v)
		if err != nil {
			return err
		}
		d.Time = t
	case []byte:
		t, err := time.Parse(DateLayout, string(v))
		if err != nil {
			return err
		}
		d.Time = t
	case time.Time:
		d.Time = v
	default:
		return fmt.Errorf("types: cannot scan %T into Date", src)
	}
	return nil
}
