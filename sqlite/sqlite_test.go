package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/SoWieMarkus/gokit/sqlite"
	"github.com/SoWieMarkus/gokit/sqlite/sqlitetest"
)

var migrations = fstest.MapFS{
	"00001_init.sql": {Data: []byte(`-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id));
-- +goose Down
DROP TABLE posts;
DROP TABLE users;
`)},
}

func TestSQLite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := sqlitetest.Open(t, migrations)

	if _, err := db.ExecContext(ctx, `INSERT INTO users (name) VALUES ('a')`); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO users (name) VALUES ('a')`)
	if !sqlite.IsUniqueViolation(err) {
		t.Fatalf("want unique violation, got %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO posts (user_id) VALUES (999)`)
	if !sqlite.IsForeignKeyViolation(err) {
		t.Fatalf("want foreign key violation, got %v", err)
	}
	var name string
	err = db.QueryRowContext(ctx, `SELECT name FROM users WHERE id = 999`).Scan(&name)
	if !sqlite.IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}

	// A failing transaction leaves nothing behind.
	boom := errors.New("boom")
	err = sqlite.InTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (name) VALUES ('b')`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rollback failed: count=%d err=%v", n, err)
	}

	// Migrating again is a no-op.
	if err := sqlite.Migrate(ctx, db, migrations); err != nil {
		t.Fatal(err)
	}
}
