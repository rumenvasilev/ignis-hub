package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
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
func (g *GCSStorage) getModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
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

func (g *GCSStorage) getModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
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
func (g *GCSStorage) getProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
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

func (g *GCSStorage) getProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
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

// CLI-specific methods for ignishubctl

// ListProviders lists all available providers in the storage
func (g *GCSStorage) listProviders(ctx context.Context) ([]ProviderInfo, error) {
	prefix := fmt.Sprintf("%s/providers/", g.prefix)

	// List all objects under the providers prefix
	bucket := g.client.Bucket(g.bucket)
	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}

	providerMap := make(map[string]*ProviderInfo)

	// Get namespaces
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list providers: %w", err)
		}

		if attrs.Prefix == "" {
			continue
		}

		// Extract namespace from prefix
		namespace := extractNamespace(attrs.Prefix, prefix)
		if namespace == "" {
			continue
		}

		// List types under this namespace
		typeQuery := &storage.Query{
			Prefix:    attrs.Prefix,
			Delimiter: "/",
		}
		typeIt := bucket.Objects(ctx, typeQuery)

		for {
			typeAttrs, err := typeIt.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				g.logger.Warn("Failed to list provider types", "namespace", namespace, "error", err)
				break
			}

			if typeAttrs.Prefix == "" {
				continue
			}

			providerType := extractTypeName(typeAttrs.Prefix, attrs.Prefix)
			if providerType == "" {
				continue
			}

			key := fmt.Sprintf("%s/%s", namespace, providerType)

			// Get versions for this provider
			metadata, err := g.getProviderVersions(ctx, namespace, providerType)
			if err != nil {
				g.logger.Warn("Failed to get provider versions", "namespace", namespace, "type", providerType, "error", err)
				providerMap[key] = &ProviderInfo{
					Namespace: namespace,
					Type:      providerType,
					Versions:  []string{},
				}
				continue
			}

			versions := make([]string, 0, len(metadata.Versions))
			for _, v := range metadata.Versions {
				versions = append(versions, v.Version)
			}

			providerMap[key] = &ProviderInfo{
				Namespace: namespace,
				Type:      providerType,
				Versions:  versions,
			}
		}
	}

	// Convert map to slice
	providers := make([]ProviderInfo, 0, len(providerMap))
	for _, info := range providerMap {
		providers = append(providers, *info)
	}

	return providers, nil
}

// ListModules lists all available modules in the storage
// TODO: Refactor this to use a more efficient algorithm
//
//nolint:gocyclo
func (g *GCSStorage) listModules(ctx context.Context) ([]ModuleInfo, error) {
	prefix := fmt.Sprintf("%s/modules/", g.prefix)

	bucket := g.client.Bucket(g.bucket)
	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}

	moduleMap := make(map[string]*ModuleInfo)

	// Get namespaces
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list modules: %w", err)
		}

		if attrs.Prefix == "" {
			continue
		}

		namespace := extractNamespace(attrs.Prefix, prefix)
		if namespace == "" {
			continue
		}

		// List module names under this namespace
		nameQuery := &storage.Query{
			Prefix:    attrs.Prefix,
			Delimiter: "/",
		}
		nameIt := bucket.Objects(ctx, nameQuery)

		for {
			nameAttrs, err := nameIt.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				g.logger.Warn("Failed to list module names", "namespace", namespace, "error", err)
				break
			}

			if nameAttrs.Prefix == "" {
				continue
			}

			moduleName := extractTypeName(nameAttrs.Prefix, attrs.Prefix)
			if moduleName == "" {
				continue
			}

			// List systems under this module
			systemQuery := &storage.Query{
				Prefix:    nameAttrs.Prefix,
				Delimiter: "/",
			}
			systemIt := bucket.Objects(ctx, systemQuery)

			for {
				systemAttrs, err := systemIt.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					g.logger.Warn("Failed to list module systems", "namespace", namespace, "name", moduleName, "error", err)
					break
				}

				if systemAttrs.Prefix == "" {
					continue
				}

				system := extractTypeName(systemAttrs.Prefix, nameAttrs.Prefix)
				if system == "" {
					continue
				}

				key := fmt.Sprintf("%s/%s/%s", namespace, moduleName, system)

				// Get versions for this module
				metadata, err := g.getModuleVersions(ctx, namespace, moduleName, system)
				if err != nil {
					g.logger.Warn("Failed to get module versions", "namespace", namespace, "name", moduleName, "system", system, "error", err)
					moduleMap[key] = &ModuleInfo{
						Namespace: namespace,
						Name:      moduleName,
						System:    system,
						Versions:  []string{},
					}
					continue
				}

				versions := make([]string, 0, len(metadata.Versions))
				for _, v := range metadata.Versions {
					versions = append(versions, v.Version)
				}

				moduleMap[key] = &ModuleInfo{
					Namespace: namespace,
					Name:      moduleName,
					System:    system,
					Versions:  versions,
				}
			}
		}
	}

	// Convert map to slice
	modules := make([]ModuleInfo, 0, len(moduleMap))
	for _, info := range moduleMap {
		modules = append(modules, *info)
	}

	return modules, nil
}

// UploadProviderBinary uploads a provider binary to storage
func (g *GCSStorage) uploadProviderBinary(ctx context.Context, namespace, typeName, version, osName, arch, filepath string) error {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for filename
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Upload to GCS
	filename := fileInfo.Name()
	key := fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/%s", g.prefix, namespace, typeName, version, osName, arch, filename)

	obj := g.client.Bucket(g.bucket).Object(key)
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to upload to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	g.logger.Info("Uploaded provider binary", "key", key, "size", fileInfo.Size())
	return nil
}

// UploadProviderMetadata uploads provider metadata to storage
func (g *GCSStorage) uploadProviderMetadata(ctx context.Context, namespace, typeName string, metadata *models.ProviderMetadata) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload to GCS
	key := g.getProviderMetadataKey(namespace, typeName)

	obj := g.client.Bucket(g.bucket).Object(key)
	writer := obj.NewWriter(ctx)
	writer.ContentType = "application/json"

	if _, err := writer.Write(metadataJSON); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to upload metadata to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	g.logger.Info("Uploaded provider metadata", "key", key)
	return nil
}

// UploadModuleArchive uploads a module archive to storage
func (g *GCSStorage) uploadModuleArchive(ctx context.Context, namespace, name, system, version, filepath string) error {
	// TODO: Implement module archive upload
	return fmt.Errorf("UploadModuleArchive not yet implemented for GCS")
}

// UploadModuleMetadata uploads module metadata to storage
func (g *GCSStorage) uploadModuleMetadata(ctx context.Context, namespace, name, system string, metadata *models.ModuleMetadata) error {
	// TODO: Implement module metadata upload
	return fmt.Errorf("UploadModuleMetadata not yet implemented for GCS")
}

// DeleteProvider deletes a provider version from storage
func (g *GCSStorage) deleteProvider(ctx context.Context, namespace, typeName, version string) error {
	// TODO: Implement provider deletion
	return fmt.Errorf("DeleteProvider not yet implemented for GCS")
}

// DeleteModule deletes a module version from storage
func (g *GCSStorage) deleteModule(ctx context.Context, namespace, name, system, version string) error {
	// TODO: Implement module deletion
	return fmt.Errorf("DeleteModule not yet implemented for GCS")
}

// New unified interface methods

// GetVersions implements Reader.GetVersions
func (g *GCSStorage) GetVersions(ctx context.Context, id ResourceIdentifier) (*VersionsResponse, error) {
	switch id.Type {
	case ResourceTypeModule:
		metadata, err := g.getModuleVersions(ctx, id.Namespace, id.Name, id.System)
		if err != nil {
			return nil, err
		}
		return &VersionsResponse{Module: metadata}, nil
	case ResourceTypeProvider:
		metadata, err := g.getProviderVersions(ctx, id.Namespace, id.Name)
		if err != nil {
			return nil, err
		}
		return &VersionsResponse{Provider: metadata}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// GetDownloadInfo implements Reader.GetDownloadInfo
func (g *GCSStorage) GetDownloadInfo(ctx context.Context, id ResourceIdentifier) (*DownloadInfoResponse, error) {
	switch id.Type {
	case ResourceTypeModule:
		url, err := g.getModuleDownloadURL(ctx, id.Namespace, id.Name, id.System, id.Version)
		if err != nil {
			return nil, err
		}
		return &DownloadInfoResponse{URL: url}, nil
	case ResourceTypeProvider:
		metadata, err := g.getProviderBinary(ctx, id.Namespace, id.Name, id.Version, id.OS, id.Arch)
		if err != nil {
			return nil, err
		}
		return &DownloadInfoResponse{ProviderBinary: metadata}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// List implements Reader.List
//
//nolint:dupl
func (g *GCSStorage) List(ctx context.Context, resourceType ResourceType) ([]ResourceInfo, error) {
	switch resourceType {
	case ResourceTypeModule:
		modules, err := g.listModules(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]ResourceInfo, len(modules))
		for i, m := range modules {
			result[i] = ResourceInfo{
				ResourceType: ResourceTypeModule,
				Namespace:    m.Namespace,
				Name:         m.Name,
				System:       m.System,
				Versions:     m.Versions,
			}
		}
		return result, nil
	case ResourceTypeProvider:
		providers, err := g.listProviders(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]ResourceInfo, len(providers))
		for i, p := range providers {
			result[i] = ResourceInfo{
				ResourceType: ResourceTypeProvider,
				Namespace:    p.Namespace,
				Name:         p.Type,
				Versions:     p.Versions,
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// Upload implements Writer.Upload
func (g *GCSStorage) Upload(ctx context.Context, id ResourceIdentifier, filepath string) error {
	switch id.Type {
	case ResourceTypeModule:
		return g.uploadModuleArchive(ctx, id.Namespace, id.Name, id.System, id.Version, filepath)
	case ResourceTypeProvider:
		return g.uploadProviderBinary(ctx, id.Namespace, id.Name, id.Version, id.OS, id.Arch, filepath)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// UpdateMetadata implements Writer.UpdateMetadata
func (g *GCSStorage) UpdateMetadata(ctx context.Context, id ResourceIdentifier, metadata interface{}) error {
	switch id.Type {
	case ResourceTypeModule:
		moduleMeta, ok := metadata.(*models.ModuleMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for module: expected *models.ModuleMetadata")
		}
		return g.uploadModuleMetadata(ctx, id.Namespace, id.Name, id.System, moduleMeta)
	case ResourceTypeProvider:
		providerMeta, ok := metadata.(*models.ProviderMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for provider: expected *models.ProviderMetadata")
		}
		return g.uploadProviderMetadata(ctx, id.Namespace, id.Name, providerMeta)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// Delete implements Writer.Delete
func (g *GCSStorage) Delete(ctx context.Context, id ResourceIdentifier) error {
	switch id.Type {
	case ResourceTypeModule:
		return g.deleteModule(ctx, id.Namespace, id.Name, id.System, id.Version)
	case ResourceTypeProvider:
		return g.deleteProvider(ctx, id.Namespace, id.Name, id.Version)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// Helper functions for path parsing

// extractNamespace extracts the namespace from a prefix path
// e.g., "prefix/providers/hashicorp/" -> "hashicorp"
func extractNamespace(prefix, basePrefix string) string {
	rel := strings.TrimPrefix(prefix, basePrefix)
	rel = strings.TrimSuffix(rel, "/")
	return rel
}

// extractTypeName extracts the type/name from a prefix path
// e.g., "prefix/providers/hashicorp/aws/" with base "prefix/providers/hashicorp/" -> "aws"
func extractTypeName(prefix, basePrefix string) string {
	rel := strings.TrimPrefix(prefix, basePrefix)
	rel = strings.TrimSuffix(rel, "/")
	return rel
}
