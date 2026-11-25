// Package sqlite opens SQLite databases with production ready settings, runs
// goose migrations and provides transaction and error helpers. It uses the
// pure Go driver modernc.org/sqlite, no cgo needed.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"strings"

	"github.com/pressly/goose/v3"
	// Pure Go SQLite driver, no cgo needed.
	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path, creating it if needed.
//
//   - WAL lets readers work while a write is in progress.
//   - busy_timeout makes writers queue up instead of failing with SQLITE_BUSY.
//   - _txlock=immediate takes the write lock at BEGIN, which serializes all
//     transactions and avoids deadlocks from lock upgrades.
//   - _time_format=sqlite stores times as sortable "YYYY-MM-DD HH:MM:SS" text.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "busy_timeout(10000)")
	params.Set("_txlock", "immediate")
	params.Set("_time_format", "sqlite")

	db, err := sql.Open("sqlite", "file:"+path+"?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Migrate applies all pending goose migrations found at the root of files.
// Pass the result of fs.Sub when the migrations live in a subdirectory of an
// embed.FS.
func Migrate(ctx context.Context, db *sql.DB, files fs.FS) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		slog.InfoContext(ctx, "applied migration", "version", r.Source.Version, "duration", r.Duration)
	}
	return nil
}

// InTx runs fn inside a transaction. The transaction is committed when fn
// returns nil and rolled back otherwise.
//
// With Open's _txlock=immediate every transaction takes SQLite's write lock
// up front, so transactions are fully serialized.
func InTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback after a successful commit is a no-op.
	defer tx.Rollback() //nolint:errcheck

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// IsNotFound reports whether err means "no row matched".
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// IsUniqueViolation reports whether err is a UNIQUE constraint failure.
// modernc.org/sqlite reports constraint errors only through their message.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// IsForeignKeyViolation reports whether err is a FOREIGN KEY constraint failure.
func IsForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}
