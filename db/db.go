// Package db holds recurin's database schema and SQL.
//
//   - migrations/ are goose migrations, embedded in the binary and applied by
//     `recurin migrate`.
//   - queries/<module>/ are the SQL queries each module runs; sqlc turns them
//     into type-safe Go in internal/<module>/<module>db (or
//     internal/platform/platformdb for platform tables).
//
// See docs/database.md for the conventions every table follows.
package db

import (
	"embed"
	"io/fs"
)

//go:generate sqlc generate -f ../sqlc.yaml

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the migration files, named NNNNN_description.sql.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err) // the directory is embedded at compile time, so it always exists
	}
	return sub
}
