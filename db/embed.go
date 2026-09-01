// Package db embeds the SQL migrations so the binary carries its own schema.
// The migrations are the source of truth (CLAUDE.md §4); there is no
// automigrate anywhere in this system.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
