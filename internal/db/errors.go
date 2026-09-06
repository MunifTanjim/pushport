package db

import (
	"errors"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// IsForeignKeyViolation reports whether err is a SQLite foreign-key constraint
// failure — e.g. deleting a usage plan an app or instance is still assigned to.
func IsForeignKeyViolation(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		return se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
	}
	return false
}

// IsUniqueViolation reports whether err is a SQLite unique-constraint failure —
// e.g. a second app-scoped (or instance-scoped) usage plan for the same app or
// instance.
func IsUniqueViolation(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		return se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
