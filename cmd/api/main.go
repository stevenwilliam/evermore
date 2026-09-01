// Command api is Evermore's single binary: HTTP server plus the operational
// subcommands. CLAUDE.md §2 wants a thin entrypoint that wires and runs.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/stevenwilliam/evermore/db"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "evermore: %v\n", err)
		os.Exit(1)
	}
}

func usage() string {
	return `evermore — usage:
  api serve            run the HTTP server
  api migrate          apply every pending migration
  api migrate:status   show applied and pending migrations
  api migrate:down     roll back the most recent migration (development only)
  api seed             load demo data
`
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Print(usage())
		return nil
	}

	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch os.Args[1] {
	case "migrate":
		return cmdMigrate(ctx, cfg)
	case "migrate:status":
		return cmdMigrateStatus(ctx, cfg)
	case "migrate:down":
		return cmdMigrateDown(ctx, cfg)
	case "serve":
		return cmdServe(ctx, cfg)
	case "seed":
		return cmdSeed(ctx, cfg)
	default:
		fmt.Print(usage())
		return fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
}

func openDB(ctx context.Context, cfg *config.Config) (*database.Options, error) {
	return &database.Options{DSN: cfg.DatabaseURL}, nil
}

func cmdMigrate(ctx context.Context, cfg *config.Config) error {
	migrations, err := database.Load(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx, database.Options{DSN: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer conn.Close()

	applied, err := database.Up(ctx, conn, migrations)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Println("migrate: nothing pending, schema is up to date")
		return nil
	}
	for _, v := range applied {
		fmt.Printf("migrate: applied %04d\n", v)
	}
	fmt.Printf("migrate: %d migration(s) applied\n", len(applied))
	return nil
}

func cmdMigrateStatus(ctx context.Context, cfg *config.Config) error {
	migrations, err := database.Load(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx, database.Options{DSN: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer conn.Close()

	applied, pending, err := database.Status(ctx, conn, migrations)
	if err != nil {
		return err
	}
	fmt.Printf("applied (%d):\n", len(applied))
	for _, a := range applied {
		fmt.Printf("  %04d_%-24s %s\n", a.Version, a.Name, a.AppliedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("pending (%d):\n", len(pending))
	for _, p := range pending {
		fmt.Printf("  %04d_%s\n", p.Version, p.Name)
	}
	return nil
}

func cmdMigrateDown(ctx context.Context, cfg *config.Config) error {
	if cfg.IsProduction() {
		return fmt.Errorf("migrations are forward-only in production (CLAUDE.md §4)")
	}
	migrations, err := database.Load(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	conn, err := database.Open(ctx, database.Options{DSN: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer conn.Close()

	v, err := database.Down(ctx, conn, migrations)
	if err != nil {
		return err
	}
	fmt.Printf("migrate: rolled back %04d\n", v)
	return nil
}
