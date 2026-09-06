package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"

	"github.com/MunifTanjim/pushport/internal/db/sqlc"
)

var ErrNotFound = errors.New("store: not found")

//go:embed migrations/*.sql
var migrationFS embed.FS

// resolvePath maps a database source (PUSHPORT_DATABASE_URI form, bare path, or
// ":memory:") to a SQLite filesystem path.
func resolvePath(source string) (string, error) {
	if !strings.Contains(source, "://") {
		return source, nil
	}
	u, err := url.Parse(source)
	if err != nil {
		return "", fmt.Errorf("db: invalid database uri %q: %w", source, err)
	}
	if u.Scheme != "sqlite" {
		return "", fmt.Errorf("db: unsupported database scheme %q (only sqlite is supported)", u.Scheme)
	}
	if u.Host != "" && u.Host != "." {
		return "", fmt.Errorf("db: invalid sqlite uri %q: path must be absolute (sqlite:///path) or relative (sqlite://./path)", source)
	}
	path := u.Host + u.Path
	if path == "" {
		return "", fmt.Errorf("db: sqlite uri %q has no path", source)
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path, nil
}

func isMemory(path string) bool {
	return path == ":memory:" || strings.HasPrefix(path, "file::memory:") || strings.Contains(path, "mode=memory")
}

// dsn appends connection pragmas. _txlock=immediate makes read-write transactions
// take the writer lock at BEGIN (see DB.InTx) instead of the default deferred lock.
func dsn(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	q := "_pragma=foreign_keys(1)"
	if !isMemory(path) {
		q += "&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	}
	return path + sep + q
}

type DB struct {
	*sqlc.Queries
	sql *sql.DB
}

// Open opens (or creates) the database named by source and runs pending
// migrations. Callers should defer Close.
func Open(source string) (*DB, error) {
	path, err := resolvePath(source)
	if err != nil {
		return nil, err
	}
	if !isMemory(path) {
		// Strip the DSN query before deriving the directory.
		file := path
		if i := strings.IndexByte(file, '?'); i >= 0 {
			file = file[:i]
		}
		if dir := filepath.Dir(file); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("db: create database directory %q: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	if isMemory(path) {
		db.SetMaxOpenConns(1) // in-memory DB lives in one connection
	} else {
		db.SetMaxOpenConns(8)
		db.SetMaxIdleConns(8)
	}

	// Run migrations via the same *sql.DB so that :memory: tests see the schema.
	src, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		_ = db.Close()
		return nil, err
	}

	return &DB{Queries: sqlc.New(db), sql: db}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

// InTx runs fn inside a single read-write transaction. Because the DSN sets
// _txlock=immediate, concurrent read-modify-writes serialize (no lost updates).
// fn receives a *Queries bound to the transaction; a non-nil return (or panic)
// rolls back, nil commits.
func (d *DB) InTx(ctx context.Context, fn func(*sqlc.Queries) error) (err error) {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(d.Queries.WithTx(tx)); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}
