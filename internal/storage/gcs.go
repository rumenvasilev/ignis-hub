package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	registryConfig "github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
)

type GCSStorage struct {
	client *storage.Client
	bucket string
	prefix string
	logger *slog.Logger
}

func newGCSStorage(ctx context.Context, cfg *registryConfig.Config, logger *slog.Logger) (*GCSStorage, error) {
	var opts []option.ClientOption

	// Use custom endpoint for local development
	if cfg.GCP.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(cfg.GCP.Endpoint))
		logger.Debug("Using custom GCS endpoint for local development")
	}

	// Use explicit credentials file if provided
	if cfg.GCP.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.GCP.CredentialsFile))
		logger.Debug("Using explicit credentials file", "file", cfg.GCP.CredentialsFile)
	}
	// Otherwise, uses Application Default Credentials (ADC)

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSStorage{
		client: client,
		bucket: cfg.GCP.GCSBucket,
		prefix: cfg.GCP.GCSPrefix,
		logger: logger,
	}, nil
}

// Module methods
func (g *GCSStorage) GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
	key := g.getModuleMetadataKey(namespace, name, system)

	obj := g.client.Bucket(g.bucket).Object(key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		g.logger.Error("Failed to get module metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %w", ErrModuleNotFound, err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read module metadata: %w", err)
	}

	var metadata models.ModuleMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal module metadata: %w", err)
	}

	return &metadata, nil
}

func (g *GCSStorage) GetModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	key := g.getModuleArchiveKey(namespace, name, system, version)

	// Check if the module version exists
	obj := g.client.Bucket(g.bucket).Object(key)
	if _, err := obj.Attrs(ctx); err != nil {
		g.logger.Error("Module version not found",
			"error", err,
			"namespace", namespace,
			"name", name,
			"system", system,
			"version", version)
		return "", fmt.Errorf("%w: %w", ErrModuleVersionNotFound, err)
	}

	// Generate a signed URL for download (valid for 1 hour)
	url, err := g.client.Bucket(g.bucket).SignedURL(key, &storage.SignedURLOptions{
		Method: "GET",
		// Expires: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}

// Provider methods
func (g *GCSStorage) GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
	key := g.getProviderMetadataKey(namespace, typeName)

	obj := g.client.Bucket(g.bucket).Object(key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		g.logger.Error("Failed to get provider metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %w", ErrProviderNotFound, err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider metadata: %w", err)
	}

	var metadata models.ProviderMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider metadata: %w", err)
	}

	return &metadata, nil
}

func (g *GCSStorage) GetProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
	key := g.getProviderBinaryMetadataKey(namespace, typeName, version, os, arch)

	obj := g.client.Bucket(g.bucket).Object(key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		g.logger.Error("Failed to get provider binary metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %w", ErrProviderBinaryNotFound, err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider binary metadata: %w", err)
	}

	var metadata models.ProviderBinaryMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider binary metadata: %w", err)
	}

	return &metadata, nil
}

// Key generation methods
func (g *GCSStorage) getModuleMetadataKey(namespace, name, system string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/metadata.json", g.prefix, namespace, name, system)
}

func (g *GCSStorage) getModuleArchiveKey(namespace, name, system, version string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/%s/archive.tar.gz", g.prefix, namespace, name, system, version)
}

func (g *GCSStorage) getProviderMetadataKey(namespace, typeName string) string {
	return fmt.Sprintf("%s/providers/%s/%s/metadata.json", g.prefix, namespace, typeName)
}

func (g *GCSStorage) getProviderBinaryMetadataKey(namespace, typeName, version, os, arch string) string {
	return fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/metadata.json", g.prefix, namespace, typeName, version, os, arch)
}

// Health check method
func (g *GCSStorage) HealthCheck(ctx context.Context) error {
	// Check if we can access the bucket
	bucket := g.client.Bucket(g.bucket)
	if _, err := bucket.Attrs(ctx); err != nil {
		return fmt.Errorf("GCS bucket health check failed: %w", err)
	}
	return nil
}
