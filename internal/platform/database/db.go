package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver
)

// Options configures the pool.
type Options struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Open connects and verifies the connection actually works. A pool that has
// never round-tripped is not a working connection, so this pings before
// returning.
func Open(ctx context.Context, opt Options) (*sql.DB, error) {
	if opt.MaxOpenConns == 0 {
		opt.MaxOpenConns = 25
	}
	if opt.MaxIdleConns == 0 {
		opt.MaxIdleConns = 5
	}
	if opt.ConnMaxLifetime == 0 {
		opt.ConnMaxLifetime = time.Hour
	}
	if opt.ConnMaxIdleTime == 0 {
		opt.ConnMaxIdleTime = 10 * time.Minute
	}

	db, err := sql.Open("pgx", opt.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	db.SetMaxOpenConns(opt.MaxOpenConns)
	db.SetMaxIdleConns(opt.MaxIdleConns)
	db.SetConnMaxLifetime(opt.ConnMaxLifetime)
	db.SetConnMaxIdleTime(opt.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	// Every timestamp this system stores is UTC (CLAUDE.md §4). Business-day
	// logic converts to Asia/Jakarta explicitly, never relying on the server's
	// or the session's zone, so pinning the session to UTC removes one way for
	// that rule to be quietly broken.
	if _, err := db.ExecContext(pingCtx, "SET TIME ZONE 'UTC'"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: pinning session to UTC: %w", err)
	}
	return db, nil
}

// InTx runs fn inside a transaction, rolling back on error or panic.
func InTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
