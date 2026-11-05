package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"time"
)

var migrationName = regexp.MustCompile(`^[0-9]{6}_[a-z0-9_]+\.sql$`)

type migration struct {
	name     string
	checksum string
	query    string
}

func migrate(ctx context.Context, db *sql.DB, source fs.FS) error {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	var pending []migration
	versions := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !migrationName.MatchString(entry.Name()) {
			return fmt.Errorf("invalid migration file name")
		}
		version := entry.Name()[:6]
		if versions[version] {
			return fmt.Errorf("duplicate migration version")
		}
		versions[version] = true
		data, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			return fmt.Errorf("read migration: %w", err)
		}
		pending = append(pending, migration{
			name: entry.Name(), checksum: fmt.Sprintf("%x", sha256.Sum256(data)), query: string(data),
		})
	}
	if len(pending) == 0 {
		return fmt.Errorf("no migrations available")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migrations'").Scan(&exists); err != nil {
		return fmt.Errorf("check migration ledger: %w", err)
	}
	applied := make(map[string]string)
	if exists == 1 {
		rows, err := tx.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations")
		if err != nil {
			return fmt.Errorf("read migration ledger: %w", err)
		}
		for rows.Next() {
			var version, checksum string
			if err := rows.Scan(&version, &checksum); err != nil {
				rows.Close()
				return fmt.Errorf("scan migration ledger: %w", err)
			}
			applied[version] = checksum
		}
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil {
			return fmt.Errorf("iterate migration ledger: %w", rowsErr)
		}
	} else {
		var tables int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
			return fmt.Errorf("check untracked database: %w", err)
		}
		if tables > 0 {
			return fmt.Errorf("refusing to migrate an untracked nonempty database")
		}
	}

	// Applied migrations must be an unchanged prefix; reject downgrade and inserted history.
	if len(applied) > len(pending) {
		return fmt.Errorf("database has migrations unknown to this binary")
	}
	for i := 0; i < len(applied); i++ {
		checksum, ok := applied[pending[i].name]
		if !ok {
			return fmt.Errorf("migration history is not a known prefix")
		}
		if checksum != pending[i].checksum {
			return fmt.Errorf("applied migration checksum changed: %s", pending[i].name)
		}
	}
	for _, migration := range pending[len(applied):] {
		if _, err := tx.ExecContext(ctx, migration.query); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, checksum, applied_at) VALUES (?, ?, ?)",
			migration.name, migration.checksum, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}
