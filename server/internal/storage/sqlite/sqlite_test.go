package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/OpenALC/relay/server/migrations"
)

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "data", "relay # test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func migrationFiles(t *testing.T) fstest.MapFS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{}
	for _, entry := range entries {
		data, err := fs.ReadFile(migrations.Files, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	return files
}

func TestOpenAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.db")
	for range 2 {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 3 {
			t.Fatalf("migration ledger: count=%d err=%v", count, err)
		}
		for pragma, expected := range map[string]string{"journal_mode": "wal", "foreign_keys": "1", "synchronous": "2", "busy_timeout": "5000"} {
			var value string
			if err := db.QueryRow("PRAGMA " + pragma).Scan(&value); err != nil || value != expected {
				t.Fatalf("PRAGMA %s: value=%s err=%v", pragma, value, err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	db := openTestDatabase(t)
	files := migrationFiles(t)
	files["000002_broken.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE transient (id INTEGER); INSERT INTO missing_table VALUES (1);")}
	if err := migrate(context.Background(), db, files); err == nil {
		t.Fatal("expected failed migration")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE name = 'transient'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("DDL was not rolled back: count=%d err=%v", count, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 3 {
		t.Fatalf("failed migration was recorded: count=%d err=%v", count, err)
	}
}

func TestMigrationHistoryProtection(t *testing.T) {
	for _, mode := range []string{"modified", "future", "gap"} {
		t.Run(mode, func(t *testing.T) {
			db := openTestDatabase(t)
			files := migrationFiles(t)
			switch mode {
			case "modified":
				files["000001_migration_ledger.sql"].Data = append(files["000001_migration_ledger.sql"].Data, []byte("\n-- changed")...)
			case "future":
				if _, err := db.Exec("INSERT INTO schema_migrations VALUES ('000099_future.sql', ?, 'test')", strings.Repeat("a", 64)); err != nil {
					t.Fatal(err)
				}
			case "gap":
				files["000000_inserted.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
			}
			if err := migrate(context.Background(), db, files); err == nil {
				t.Fatal("expected incompatible history to be rejected")
			}
		})
	}
}

func TestDatabaseURIHandlesReservedCharacters(t *testing.T) {
	db := openTestDatabase(t)
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
}
