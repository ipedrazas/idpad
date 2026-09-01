// Package migrations embeds the SQL migration files so the server can apply
// them on start-up without shipping the .sql files alongside the binary.
// The same files are the input to the `task migrate` CLI target, keeping a
// single source of truth for the schema.
package migrations

import "embed"

// FS holds every numbered .sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
