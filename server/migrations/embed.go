// Package migrations contains immutable forward-only SQLite migrations.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
