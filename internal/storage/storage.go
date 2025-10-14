package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
)

type Storage interface {
	GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error)
	GetModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error)
	GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error)
	GetProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error)
	HealthCheck(ctx context.Context) error
}

func NewStorage(ctx context.Context, cfg *config.Config, logger *slog.Logger) (Storage, error) {
	switch cfg.Provider {
	case "aws":
		return newS3Storage(ctx, cfg, logger)
	case "gcp":
		return newGCSStorage(ctx, cfg, logger)
	}

	return nil, fmt.Errorf("invalid provider: %s", cfg.Provider)
}
