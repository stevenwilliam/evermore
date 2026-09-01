package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	httpadapter "github.com/stevenwilliam/evermore/internal/adapter/http"
	"github.com/stevenwilliam/evermore/internal/app/seed"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/logging"
	"github.com/stevenwilliam/evermore/web"
)

// buildCommit is stamped at build time with -ldflags; it falls back to asking
// git so a `go run` still reports something true rather than "unknown".
var buildCommit = ""

func commit() string {
	if buildCommit != "" {
		return buildCommit
	}
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func cmdServe(ctx context.Context, cfg *config.Config) error {
	log := logging.New(cfg.LogLevel, cfg.AppEnv)

	conn, err := database.Open(ctx, database.Options{DSN: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer conn.Close()

	router, err := httpadapter.NewRouter(httpadapter.Deps{
		DB:          conn,
		Cfg:         cfg,
		Templates:   web.Templates,
		Public:      web.Public,
		Logger:      log,
		BuildCommit: commit(),
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Log the configuration with every secret masked, so a startup line is
	// safe to paste into an issue (CLAUDE.md §7).
	redacted := cfg.Redacted()
	keys := make([]string, 0, len(redacted))
	for k := range redacted {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	attrs := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		attrs = append(attrs, k, redacted[k])
	}
	log.Info("starting evermore", append([]any{"commit", commit()}, attrs...)...)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	log.Info("listening", "addr", cfg.Addr(), "base_url", cfg.BaseURL)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
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
