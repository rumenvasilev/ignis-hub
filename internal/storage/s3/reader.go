package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

// HealthCheck implements api.HealthChecker.
func (st *Storage) HealthCheck(ctx context.Context) error {
	_, err := st.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(st.bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 bucket health check failed: %w", err)
	}
	return nil
}

// GetVersions implements api.Reader.GetVersions.
func (st *Storage) GetVersions(ctx context.Context, id api.ResourceIdentifier) (*api.VersionsResponse, error) {
	switch id.Type {
	case api.ResourceTypeModule:
		key := st.getModuleMetadataKey(id.Namespace, id.Name, id.System)
		result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			st.logger.Error("Failed to get module metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %v", api.ErrModuleNotFound, err)
		}
		defer result.Body.Close()

		body, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read module metadata: %w", err)
		}

		var metadata models.ModuleMetadata
		if err := json.Unmarshal(body, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal module metadata: %w", err)
		}
		return &api.VersionsResponse{Module: &metadata}, nil

	case api.ResourceTypeProvider:
		key := st.getProviderMetadataKey(id.Namespace, id.Name)
		result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			st.logger.Error("Failed to get provider metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %v", api.ErrProviderNotFound, err)
		}
		defer result.Body.Close()

		body, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read provider metadata: %w", err)
		}

		var metadata models.ProviderMetadata
		if err := json.Unmarshal(body, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal provider metadata: %w", err)
		}
		return &api.VersionsResponse{Provider: &metadata}, nil

	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// GetDownloadInfo implements api.Reader.GetDownloadInfo.
func (st *Storage) GetDownloadInfo(ctx context.Context, id api.ResourceIdentifier) (*api.DownloadInfoResponse, error) {
	switch id.Type {
	case api.ResourceTypeModule:
		key := st.getModuleArchiveKey(id.Namespace, id.Name, id.System, id.Version)
		// Check if the module version exists
		_, err := st.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			st.logger.Error("Module version not found", "error", err,
				"namespace", id.Namespace, "name", id.Name, "system", id.System, "version", id.Version)
			return nil, fmt.Errorf("%w: %v", api.ErrModuleVersionNotFound, err)
		}

		// Return standard S3 URL for VPC-internal access
		return &api.DownloadInfoResponse{URL: st.getObjectURL(key)}, nil

	case api.ResourceTypeProvider:
		key := st.getProviderBinaryMetadataKey(id.Namespace, id.Name, id.Version, id.OS, id.Arch)
		result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			st.logger.Error("Failed to get provider binary metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %v", api.ErrProviderBinaryNotFound, err)
		}
		defer result.Body.Close()

		body, err := io.ReadAll(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read provider binary metadata: %w", err)
		}

		var metadata models.ProviderBinaryMetadata
		if err := json.Unmarshal(body, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal provider binary metadata: %w", err)
		}
		return &api.DownloadInfoResponse{ProviderBinary: &metadata}, nil

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

// listProviders lists all available providers in the api.
func (st *Storage) listProviders(ctx context.Context) ([]api.ProviderInfo, error) {
	basePrefix := fmt.Sprintf("%s/providers/", st.prefix)

	// List all objects under providers/ - parse structure from keys
	paginator := s3.NewListObjectsV2Paginator(st.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(st.bucket),
		Prefix: aws.String(basePrefix),
	})

	// Map to deduplicate providers: "namespace/type" -> ProviderInfo
	providerMap := make(map[string]*api.ProviderInfo)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list providers: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}

			// Parse key: prefix/providers/namespace/type/...
			// We only care about metadata.json files
			if !strings.HasSuffix(*obj.Key, "/metadata.json") {
				continue
			}

			// Extract namespace and type from key
			relPath := strings.TrimPrefix(*obj.Key, basePrefix)
			parts := strings.Split(relPath, "/")
			if len(parts) < 3 { // namespace/type/metadata.json
				continue
			}

			namespace := parts[0]
			providerType := parts[1]
			key := fmt.Sprintf("%s/%s", namespace, providerType)

			if _, exists := providerMap[key]; exists {
				continue
			}

			// Get versions for this provider using existing method
			resp, err := st.GetVersions(ctx, api.ResourceIdentifier{
				Type:      api.ResourceTypeProvider,
				Namespace: namespace,
				Name:      providerType,
			})
			if err != nil {
				st.logger.Warn("Failed to get provider versions", "namespace", namespace, "type", providerType, "error", err)
				providerMap[key] = &api.ProviderInfo{
					Namespace: namespace,
					Type:      providerType,
					Versions:  []string{},
				}
				continue
			}

			versions := make([]string, 0, len(resp.Provider.Versions))
			for _, v := range resp.Provider.Versions {
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
func (st *Storage) listModules(ctx context.Context) ([]api.ModuleInfo, error) {
	basePrefix := fmt.Sprintf("%s/modules/", st.prefix)

	// List all objects under modules/ - parse structure from keys
	paginator := s3.NewListObjectsV2Paginator(st.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(st.bucket),
		Prefix: aws.String(basePrefix),
	})

	// Map to deduplicate modules: "namespace/name/system" -> ModuleInfo
	moduleMap := make(map[string]*api.ModuleInfo)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list modules: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}

			// Parse key: prefix/modules/namespace/name/system/...
			// We only care about metadata.json files
			if !strings.HasSuffix(*obj.Key, "/metadata.json") {
				continue
			}

			// Extract namespace, name, and system from key
			relPath := strings.TrimPrefix(*obj.Key, basePrefix)
			parts := strings.Split(relPath, "/")
			if len(parts) < 4 { // namespace/name/system/metadata.json
				continue
			}

			namespace := parts[0]
			moduleName := parts[1]
			system := parts[2]
			key := fmt.Sprintf("%s/%s/%s", namespace, moduleName, system)

			if _, exists := moduleMap[key]; exists {
				continue
			}

			// Get versions for this module using existing method
			resp, err := st.GetVersions(ctx, api.ResourceIdentifier{
				Type:      api.ResourceTypeModule,
				Namespace: namespace,
				Name:      moduleName,
				System:    system,
			})
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

			versions := make([]string, 0, len(resp.Module.Versions))
			for _, v := range resp.Module.Versions {
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

	// Convert map to slice
	modules := make([]api.ModuleInfo, 0, len(moduleMap))
	for _, info := range moduleMap {
		modules = append(modules, *info)
	}

	return modules, nil
}
