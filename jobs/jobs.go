// Package jobs runs periodic background work next to the HTTP servers.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

// Every runs job once right away and then at every interval until ctx ends.
// Job errors are logged, they never stop the server. It always returns nil so
// it can run in an errgroup without tearing the group down.
func Every(ctx context.Context, interval time.Duration, name string, job func(context.Context) error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := job(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "background job failed", "job", name, "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
