// Package storage provides a factory function to create storage backends.
// Types and interfaces are defined in storage/api.
package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
	"github.com/rumenvasilev/ignis-hub/internal/storage/gcs"
	"github.com/rumenvasilev/ignis-hub/internal/storage/s3"
)

// New creates a new Storage based on the configured provider.
func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (api.Storage, error) {
	switch cfg.Provider {
	case "aws":
		return s3.New(ctx, cfg, logger)
	case "gcp":
		return gcs.New(ctx, cfg, logger)
	default:
		return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
	}
}

// Compile-time interface checks.
var (
	_ api.Storage = (*s3.Storage)(nil)
	_ api.Storage = (*gcs.Storage)(nil)
)
