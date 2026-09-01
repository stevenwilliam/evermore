package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/stevenwilliam/evermore/internal/app/seed"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/database"
)

// cmdServe is filled in once the HTTP layer lands; keeping the subcommand
// present from the start means the CLI surface does not change later.
func cmdServe(ctx context.Context, cfg *config.Config) error {
	return fmt.Errorf("serve: not wired yet")
}

func cmdSeed(ctx context.Context, cfg *config.Config) error {
	conn, err := database.Open(ctx, database.Options{DSN: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer conn.Close()

	res, err := seed.Run(ctx, conn, cfg.Location, time.Now())
	if err != nil {
		return err
	}
	tables := make([]string, 0, len(res.Counts))
	for t := range res.Counts {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		if res.Counts[t] > 0 {
			fmt.Printf("seed: %-24s %d row(s)\n", t, res.Counts[t])
		}
	}

	// Report what was verified by reading it back, not what was attempted.
	checks, err := seed.Verify(ctx, conn)
	if err != nil {
		return fmt.Errorf("seed applied but verification failed: %w", err)
	}
	names := make([]string, 0, len(checks))
	for n := range checks {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Println("seed: verified by reading back —")
	for _, n := range names {
		fmt.Printf("  %-20s %d\n", n, checks[n])
	}
	return nil
}
