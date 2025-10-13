package storage

import (
	"context"

	"github.com/rumenvasilev/ignis-hub/internal/models"
)

type Storage interface {
	GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error)
	GetModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error)
	GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error)
	GetProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error)
	HealthCheck(ctx context.Context) error
}
