// Package sqlitetest provides real SQLite databases for tests. Every call gets
// its own file in a temp directory, so tests are isolated and can run in
// parallel with nothing to set up.
package sqlitetest

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/SoWieMarkus/gokit/sqlite"
)

// Open opens a fresh database with all migrations from files applied. It is
// closed when the test ends.
func Open(t testing.TB, migrations fs.FS) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(ctx, db, migrations); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
