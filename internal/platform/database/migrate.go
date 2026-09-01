// Package database owns the connection pool and the migration runner.
//
// Migrations are numbered, forward-only in production, embedded via go:embed,
// and applied inside a transaction each. The migrations are the source of
// truth for the schema — CLAUDE.md §4 — and nothing in this system calls
// AutoMigrate.
package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migration is one numbered step.
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
	// Checksum of the up SQL. An already-applied migration whose file has
	// changed is refused rather than silently ignored: the database and the
	// repository would otherwise disagree about what the schema is.
	Checksum string
}

var fileRE = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Load reads the migrations out of an embedded filesystem.
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("database: reading %s: %w", dir, err)
	}

	byVersion := map[int]*Migration{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := fileRE.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("database: %s does not match NNNN_name.(up|down).sql", e.Name())
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		mig := byVersion[version]
		if mig == nil {
			mig = &Migration{Version: version, Name: m[2]}
			byVersion[version] = mig
		}
		if mig.Name != m[2] {
			return nil, fmt.Errorf("database: version %04d has two names, %q and %q", version, mig.Name, m[2])
		}
		if m[3] == "up" {
			mig.UpSQL = string(body)
			sum := sha256.Sum256(body)
			mig.Checksum = hex.EncodeToString(sum[:])
		} else {
			mig.DownSQL = string(body)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		// CLAUDE.md §4: each migration ships with a matching .down.sql. A
		// missing one is refused here rather than discovered during a rollback.
		if m.UpSQL == "" {
			return nil, fmt.Errorf("database: migration %04d_%s has no .up.sql", m.Version, m.Name)
		}
		if m.DownSQL == "" {
			return nil, fmt.Errorf("database: migration %04d_%s has no .down.sql", m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })

	// Versions must be contiguous from 1. A gap means a migration was deleted
	// or never committed, and applying the rest would produce a schema nobody
	// has ever tested.
	for i, m := range out {
		if m.Version != i+1 {
			return nil, fmt.Errorf("database: migration versions are not contiguous: expected %04d, found %04d_%s", i+1, m.Version, m.Name)
		}
	}
	return out, nil
}

const schemaTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migration (
    version     int PRIMARY KEY,
    name        text NOT NULL,
    checksum    text NOT NULL,
    applied_at  timestamptz NOT NULL DEFAULT now(),
    duration_ms int NOT NULL DEFAULT 0
)`

// Applied is one row of the migration history.
type Applied struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// Status reports what has been applied and what is pending.
func Status(ctx context.Context, db *sql.DB, migrations []Migration) (applied []Applied, pending []Migration, err error) {
	if _, err = db.ExecContext(ctx, schemaTableDDL); err != nil {
		return nil, nil, fmt.Errorf("database: creating schema_migration: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum, applied_at FROM schema_migration ORDER BY version`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	seen := map[int]Applied{}
	for rows.Next() {
		var a Applied
		if err := rows.Scan(&a.Version, &a.Name, &a.Checksum, &a.AppliedAt); err != nil {
			return nil, nil, err
		}
		// A scan into a column that does not exist leaves the zero value
		// rather than erroring, so assert something actually came back.
		if a.Version == 0 {
			return nil, nil, errors.New("database: schema_migration row scanned as version 0")
		}
		seen[a.Version] = a
		applied = append(applied, a)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	for _, m := range migrations {
		a, ok := seen[m.Version]
		if !ok {
			pending = append(pending, m)
			continue
		}
		if a.Checksum != m.Checksum {
			return nil, nil, fmt.Errorf(
				"database: migration %04d_%s was applied on %s with a different checksum — "+
					"the file has been edited since. Migrations are forward-only: add a new one",
				m.Version, m.Name, a.AppliedAt.Format(time.RFC3339))
		}
	}
	return applied, pending, nil
}

// Up applies every pending migration, each in its own transaction, in order.
// It returns the versions it applied.
func Up(ctx context.Context, db *sql.DB, migrations []Migration) ([]int, error) {
	_, pending, err := Status(ctx, db, migrations)
	if err != nil {
		return nil, err
	}

	var done []int
	for _, m := range pending {
		start := time.Now()
		if err := applyOne(ctx, db, m); err != nil {
			return done, fmt.Errorf("database: migration %04d_%s failed: %w", m.Version, m.Name, err)
		}
		elapsed := int(time.Since(start).Milliseconds())
		if _, err := db.ExecContext(ctx,
			`UPDATE schema_migration SET duration_ms = $1 WHERE version = $2`,
			elapsed, m.Version); err != nil {
			return done, err
		}
		done = append(done, m.Version)
	}
	return done, nil
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.UpSQL); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migration (version, name, checksum) VALUES ($1, $2, $3)`,
		m.Version, m.Name, m.Checksum)
	if err != nil {
		return err
	}
	// Assert the bookkeeping row actually landed. An INSERT that affected no
	// rows would leave the schema changed but unrecorded, and the next run
	// would try to apply it again.
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("recording the migration affected %d rows, want 1", n)
	}
	return tx.Commit()
}

// Down rolls back the single most recently applied migration. It exists for
// development; production is forward-only (CLAUDE.md §4).
func Down(ctx context.Context, db *sql.DB, migrations []Migration) (int, error) {
	applied, _, err := Status(ctx, db, migrations)
	if err != nil {
		return 0, err
	}
	if len(applied) == 0 {
		return 0, errors.New("database: nothing to roll back")
	}
	last := applied[len(applied)-1]

	var target *Migration
	for i := range migrations {
		if migrations[i].Version == last.Version {
			target = &migrations[i]
			break
		}
	}
	if target == nil {
		return 0, fmt.Errorf("database: migration %04d is applied but its files are missing", last.Version)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, target.DownSQL); err != nil {
		return 0, fmt.Errorf("database: rolling back %04d_%s: %w", target.Version, target.Name, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migration WHERE version = $1`, target.Version); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return target.Version, nil
}

// SplitStatements is a small helper for tests that need to run a migration
// statement by statement. It is deliberately naive — it does not parse SQL —
// and is not used by the runner, which sends each file whole.
func SplitStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
