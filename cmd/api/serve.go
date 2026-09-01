package main

import (
	"context"
	"fmt"

	"github.com/stevenwilliam/evermore/internal/platform/config"
)

// cmdServe is filled in once the HTTP layer lands; keeping the subcommand
// present from the start means the CLI surface does not change later.
func cmdServe(ctx context.Context, cfg *config.Config) error {
	return fmt.Errorf("serve: not wired yet")
}

func cmdSeed(ctx context.Context, cfg *config.Config) error {
	return fmt.Errorf("seed: not wired yet")
}
