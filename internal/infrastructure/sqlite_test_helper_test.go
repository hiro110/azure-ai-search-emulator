package infrastructure

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// schemaSQL mirrors the table creation block in main.go so that
// infrastructure tests behave identically to production.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS indexes (
    name TEXT PRIMARY KEY,
    schema TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS documents (
    index_name TEXT NOT NULL,
    key TEXT NOT NULL,
    content TEXT NOT NULL,
    PRIMARY KEY (index_name, key),
    FOREIGN KEY (index_name) REFERENCES indexes(name) ON DELETE CASCADE
);`

// newTestDB returns a fresh in-memory SQLite database with the production
// schema applied. The database is closed automatically at the end of the test
// so that no files are left on disk and no connections leak.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// "file::memory:?cache=shared" would share the DB across connections, but
	// each test wants its own isolated DB, so we use the simpler ":memory:".
	// We also constrain to a single connection so cross-statement state is
	// observable when running with `-race`.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}
