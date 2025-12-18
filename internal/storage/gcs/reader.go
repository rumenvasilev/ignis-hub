package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	gcsstorage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

// iteratorDone is used to check for iterator completion
var iteratorDone = iterator.Done

// HealthCheck implements api.HealthChecker.
func (st *Storage) HealthCheck(ctx context.Context) error {
	// Check if we can access the bucket
	if _, err := st.client.Bucket(st.bucket).Attrs(ctx); err != nil {
		return fmt.Errorf("GCS bucket health check failed: %w", err)
	}
	return nil
}

// GetVersions implements api.Reader.GetVersions.
func (st *Storage) GetVersions(ctx context.Context, id api.ResourceIdentifier) (*api.VersionsResponse, error) {
	switch id.Type {
	case api.ResourceTypeModule:
		metadata, err := st.getModuleVersions(ctx, id.Namespace, id.Name, id.System)
		if err != nil {
			return nil, err
		}
		return &api.VersionsResponse{Module: metadata}, nil
	case api.ResourceTypeProvider:
		metadata, err := st.getProviderVersions(ctx, id.Namespace, id.Name)
		if err != nil {
			return nil, err
		}
		return &api.VersionsResponse{Provider: metadata}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// GetDownloadInfo implements api.Reader.GetDownloadInfo.
func (st *Storage) GetDownloadInfo(ctx context.Context, id api.ResourceIdentifier) (*api.DownloadInfoResponse, error) {
	switch id.Type {
	case api.ResourceTypeModule:
		url, err := st.getModuleDownloadURL(ctx, id.Namespace, id.Name, id.System, id.Version)
		if err != nil {
			return nil, err
		}
		return &api.DownloadInfoResponse{URL: url}, nil
	case api.ResourceTypeProvider:
		metadata, err := st.getProviderBinary(ctx, id.Namespace, id.Name, id.Version, id.OS, id.Arch)
		if err != nil {
			return nil, err
		}
		return &api.DownloadInfoResponse{ProviderBinary: metadata}, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// List implements api.Reader.List.
func (st *Storage) List(ctx context.Context, resourceType api.ResourceType) ([]api.ResourceInfo, error) {
	switch resourceType {
	case api.ResourceTypeModule:
		modules, err := st.listModules(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]api.ResourceInfo, len(modules))
		for i, m := range modules {
			result[i] = api.ResourceInfo{
				ResourceType: api.ResourceTypeModule,
				Namespace:    m.Namespace,
				Name:         m.Name,
				System:       m.System,
				Versions:     m.Versions,
			}
		}
		return result, nil
	case api.ResourceTypeProvider:
		providers, err := st.listProviders(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]api.ResourceInfo, len(providers))
		for i, p := range providers {
			result[i] = api.ResourceInfo{
				ResourceType: api.ResourceTypeProvider,
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

// getModuleVersions retrieves module metadata from api.
func (st *Storage) getModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
	key := st.getModuleMetadataKey(namespace, name, system)

	reader, err := st.client.Bucket(st.bucket).Object(key).NewReader(ctx)
	if err != nil {
		st.logger.Error("Failed to get module metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %v", api.ErrModuleNotFound, err)
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

// getModuleDownloadURL returns the download URL for a module archive.
func (st *Storage) getModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	key := st.getModuleArchiveKey(namespace, name, system, version)

	// Check if the module version exists
	if _, err := st.client.Bucket(st.bucket).Object(key).Attrs(ctx); err != nil {
		st.logger.Error("Module version not found",
			"error", err,
			"namespace", namespace,
			"name", name,
			"system", system,
			"version", version)
		return "", fmt.Errorf("%w: %v", api.ErrModuleVersionNotFound, err)
	}

	// Return standard GCS URL for VPC-internal access
	return st.getObjectURL(key), nil
}

// getProviderVersions retrieves provider metadata from api.
func (st *Storage) getProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
	key := st.getProviderMetadataKey(namespace, typeName)

	reader, err := st.client.Bucket(st.bucket).Object(key).NewReader(ctx)
	if err != nil {
		st.logger.Error("Failed to get provider metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %v", api.ErrProviderNotFound, err)
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

// getProviderBinary retrieves provider binary metadata from api.
func (st *Storage) getProviderBinary(ctx context.Context, namespace, typeName, version, osName, arch string) (*models.ProviderBinaryMetadata, error) {
	key := st.getProviderBinaryMetadataKey(namespace, typeName, version, osName, arch)

	reader, err := st.client.Bucket(st.bucket).Object(key).NewReader(ctx)
	if err != nil {
		st.logger.Error("Failed to get provider binary metadata", "error", err, "key", key)
		return nil, fmt.Errorf("%w: %v", api.ErrProviderBinaryNotFound, err)
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

// listProviders lists all available providers in the api.
func (st *Storage) listProviders(ctx context.Context) ([]api.ProviderInfo, error) {
	prefix := fmt.Sprintf("%s/providers/", st.prefix)

	// List all objects under the providers prefix
	bucket := st.client.Bucket(st.bucket)
	query := &gcsstorage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}

	providerMap := make(map[string]*api.ProviderInfo)

	// Get namespaces
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iteratorDone {
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
		typeQuery := &gcsstorage.Query{
			Prefix:    attrs.Prefix,
			Delimiter: "/",
		}
		typeIt := bucket.Objects(ctx, typeQuery)

		for {
			typeAttrs, err := typeIt.Next()
			if err == iteratorDone {
				break
			}
			if err != nil {
				st.logger.Warn("Failed to list provider types", "namespace", namespace, "error", err)
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
			metadata, err := st.getProviderVersions(ctx, namespace, providerType)
			if err != nil {
				st.logger.Warn("Failed to get provider versions", "namespace", namespace, "type", providerType, "error", err)
				providerMap[key] = &api.ProviderInfo{
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

			providerMap[key] = &api.ProviderInfo{
				Namespace: namespace,
				Type:      providerType,
				Versions:  versions,
			}
		}
	}

	// Convert map to slice
	providers := make([]api.ProviderInfo, 0, len(providerMap))
	for _, info := range providerMap {
		providers = append(providers, *info)
	}

	return providers, nil
}

// listModules lists all available modules in the api.
//
//nolint:gocyclo
func (st *Storage) listModules(ctx context.Context) ([]api.ModuleInfo, error) {
	prefix := fmt.Sprintf("%s/modules/", st.prefix)

	bucket := st.client.Bucket(st.bucket)
	query := &gcsstorage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}

	moduleMap := make(map[string]*api.ModuleInfo)

	// Get namespaces
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iteratorDone {
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
		nameQuery := &gcsstorage.Query{
			Prefix:    attrs.Prefix,
			Delimiter: "/",
		}
		nameIt := bucket.Objects(ctx, nameQuery)

		for {
			nameAttrs, err := nameIt.Next()
			if err == iteratorDone {
				break
			}
			if err != nil {
				st.logger.Warn("Failed to list module names", "namespace", namespace, "error", err)
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
			systemQuery := &gcsstorage.Query{
				Prefix:    nameAttrs.Prefix,
				Delimiter: "/",
			}
			systemIt := bucket.Objects(ctx, systemQuery)

			for {
				systemAttrs, err := systemIt.Next()
				if err == iteratorDone {
					break
				}
				if err != nil {
					st.logger.Warn("Failed to list module systems", "namespace", namespace, "name", moduleName, "error", err)
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
				metadata, err := st.getModuleVersions(ctx, namespace, moduleName, system)
				if err != nil {
					st.logger.Warn("Failed to get module versions", "namespace", namespace, "name", moduleName, "system", system, "error", err)
					moduleMap[key] = &api.ModuleInfo{
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

				moduleMap[key] = &api.ModuleInfo{
					Namespace: namespace,
					Name:      moduleName,
					System:    system,
					Versions:  versions,
				}
			}
		}
	}

	// Convert map to slice
	modules := make([]api.ModuleInfo, 0, len(moduleMap))
	for _, info := range moduleMap {
		modules = append(modules, *info)
	}

	return modules, nil
}

// Helper functions for path extraction

func extractNamespace(prefix, basePrefix string) string {
	rel := strings.TrimPrefix(prefix, basePrefix)
	rel = strings.TrimSuffix(rel, "/")
	return rel
}

func extractTypeName(prefix, basePrefix string) string {
	rel := strings.TrimPrefix(prefix, basePrefix)
	rel = strings.TrimSuffix(rel, "/")
	return rel
}
