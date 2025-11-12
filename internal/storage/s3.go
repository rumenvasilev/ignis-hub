package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	registryConfig "github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
)

type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string
	logger *slog.Logger
}

func newS3Storage(ctx context.Context, cfg *registryConfig.Config, logger *slog.Logger) (*S3Storage, error) {
	// Configure AWS SDK with explicit credentials first
	var awsConfig aws.Config
	var err error

	if cfg.AWS.Endpoint != "" {
		// For local development with LocalStack
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.AWS.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(credCtx context.Context) (aws.Credentials, error) {
				logger.Debug("Using explicit credentials for LocalStack")
				return aws.Credentials{
					AccessKeyID:     cfg.AWS.AccessKeyID,
					SecretAccessKey: cfg.AWS.SecretAccessKey,
					SessionToken:    cfg.AWS.SessionToken,
					Source:          "ExplicitCredentials",
				}, nil
			})),
			config.WithBaseEndpoint(cfg.AWS.Endpoint),
		)
	} else {
		// For AWS production
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.AWS.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(credCtx context.Context) (aws.Credentials, error) {
				logger.Debug("Using explicit credentials for AWS")
				return aws.Credentials{
					AccessKeyID:     cfg.AWS.AccessKeyID,
					SecretAccessKey: cfg.AWS.SecretAccessKey,
					SessionToken:    cfg.AWS.SessionToken,
					Source:          "ExplicitCredentials",
				}, nil
			})),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Configure S3 client options
	var s3Options []func(*s3.Options)

	// For LocalStack, use path-style addressing
	if cfg.AWS.Endpoint != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsConfig, s3Options...)

	return &S3Storage{
		client: client,
		bucket: cfg.AWS.S3Bucket,
		prefix: cfg.AWS.S3Prefix,
		logger: logger,
	}, nil
}

// Key generation methods (private helpers)
func (s *S3Storage) getModuleMetadataKey(namespace, name, system string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/metadata.json", s.prefix, namespace, name, system)
}

func (s *S3Storage) getModuleArchiveKey(namespace, name, system, version string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/%s/archive.tar.gz", s.prefix, namespace, name, system, version)
}

func (s *S3Storage) getProviderMetadataKey(namespace, typeName string) string {
	return fmt.Sprintf("%s/providers/%s/%s/metadata.json", s.prefix, namespace, typeName)
}

func (s *S3Storage) getProviderBinaryMetadataKey(namespace, typeName, version, os, arch string) string {
	return fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/metadata.json", s.prefix, namespace, typeName, version, os, arch)
}

// Health check method
func (s *S3Storage) HealthCheck(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 bucket health check failed: %w", err)
	}
	return nil
}

// CLI-specific methods for ignishubctl (private helpers)

// listProviders lists all available providers in the storage
func (s *S3Storage) listProviders(ctx context.Context) ([]ProviderInfo, error) {
	basePrefix := fmt.Sprintf("%s/providers/", s.prefix)

	// List all objects under providers/ - parse structure from keys
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(basePrefix),
	})

	// Map to deduplicate providers: "namespace/type" -> ProviderInfo
	providerMap := make(map[string]*ProviderInfo)

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
			resp, err := s.GetVersions(ctx, ResourceIdentifier{
				Type:      ResourceTypeProvider,
				Namespace: namespace,
				Name:      providerType,
			})
			if err != nil {
				s.logger.Warn("Failed to get provider versions", "namespace", namespace, "type", providerType, "error", err)
				providerMap[key] = &ProviderInfo{
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

// listModules lists all available modules in the storage
func (s *S3Storage) listModules(ctx context.Context) ([]ModuleInfo, error) {
	basePrefix := fmt.Sprintf("%s/modules/", s.prefix)

	// List all objects under modules/ - parse structure from keys
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(basePrefix),
	})

	// Map to deduplicate modules: "namespace/name/system" -> ModuleInfo
	moduleMap := make(map[string]*ModuleInfo)

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
			resp, err := s.GetVersions(ctx, ResourceIdentifier{
				Type:      ResourceTypeModule,
				Namespace: namespace,
				Name:      moduleName,
				System:    system,
			})
			if err != nil {
				s.logger.Warn("Failed to get module versions", "namespace", namespace, "name", moduleName, "system", system, "error", err)
				moduleMap[key] = &ModuleInfo{
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

			moduleMap[key] = &ModuleInfo{
				Namespace: namespace,
				Name:      moduleName,
				System:    system,
				Versions:  versions,
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

// uploadProviderBinary uploads a provider binary to storage
func (s *S3Storage) uploadProviderBinary(ctx context.Context, namespace, typeName, version, osName, arch, filepath string) error {
	// Open the file
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Upload to S3
	filename := fileInfo.Name()
	key := fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/%s", s.prefix, namespace, typeName, version, osName, arch, filename)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	s.logger.Info("Uploaded provider binary", "key", key, "size", fileInfo.Size())
	return nil
}

// uploadProviderMetadata uploads provider metadata to storage
func (s *S3Storage) uploadProviderMetadata(ctx context.Context, namespace, typeName string, metadata *models.ProviderMetadata) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload to S3
	key := s.getProviderMetadataKey(namespace, typeName)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(metadataJSON),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload metadata to S3: %w", err)
	}

	s.logger.Info("Uploaded provider metadata", "key", key)
	return nil
}

// uploadModuleArchive uploads a module archive to storage
func (s *S3Storage) uploadModuleArchive(ctx context.Context, namespace, name, system, version, filepath string) error {
	// TODO: Implement module archive upload
	return fmt.Errorf("UploadModuleArchive not yet implemented for S3")
}

// uploadModuleMetadata uploads module metadata to storage
func (s *S3Storage) uploadModuleMetadata(ctx context.Context, namespace, name, system string, metadata *models.ModuleMetadata) error {
	// TODO: Implement module metadata upload
	return fmt.Errorf("UploadModuleMetadata not yet implemented for S3")
}

// deleteProvider deletes a provider version from storage
func (s *S3Storage) deleteProvider(ctx context.Context, namespace, typeName, version string) error {
	// TODO: Implement provider deletion
	return fmt.Errorf("DeleteProvider not yet implemented for S3")
}

// deleteModule deletes a module version from storage
func (s *S3Storage) deleteModule(ctx context.Context, namespace, name, system, version string) error {
	// TODO: Implement module deletion
	return fmt.Errorf("DeleteModule not yet implemented for S3")
}

// New unified interface methods

// GetVersions implements Reader.GetVersions
func (s *S3Storage) GetVersions(ctx context.Context, id ResourceIdentifier) (*VersionsResponse, error) {
	switch id.Type {
	case ResourceTypeModule:
		key := s.getModuleMetadataKey(id.Namespace, id.Name, id.System)
		result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			s.logger.Error("Failed to get module metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %w", ErrModuleNotFound, err)
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
		return &VersionsResponse{Module: &metadata}, nil

	case ResourceTypeProvider:
		key := s.getProviderMetadataKey(id.Namespace, id.Name)
		result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			s.logger.Error("Failed to get provider metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %w", ErrProviderNotFound, err)
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
		return &VersionsResponse{Provider: &metadata}, nil

	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// GetDownloadInfo implements Reader.GetDownloadInfo
func (s *S3Storage) GetDownloadInfo(ctx context.Context, id ResourceIdentifier) (*DownloadInfoResponse, error) {
	switch id.Type {
	case ResourceTypeModule:
		key := s.getModuleArchiveKey(id.Namespace, id.Name, id.System, id.Version)
		// Check if the module version exists
		_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			s.logger.Error("Module version not found", "error", err,
				"namespace", id.Namespace, "name", id.Name, "system", id.System, "version", id.Version)
			return nil, fmt.Errorf("%w: %w", ErrModuleVersionNotFound, err)
		}

		// Generate a presigned URL for download
		presignClient := s3.NewPresignClient(s.client)
		request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
		}
		return &DownloadInfoResponse{URL: request.URL}, nil

	case ResourceTypeProvider:
		key := s.getProviderBinaryMetadataKey(id.Namespace, id.Name, id.Version, id.OS, id.Arch)
		result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			s.logger.Error("Failed to get provider binary metadata", "error", err, "key", key)
			return nil, fmt.Errorf("%w: %w", ErrProviderBinaryNotFound, err)
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
		return &DownloadInfoResponse{ProviderBinary: &metadata}, nil

	default:
		return nil, fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// List implements Reader.List
//
//nolint:dupl
func (s *S3Storage) List(ctx context.Context, resourceType ResourceType) ([]ResourceInfo, error) {
	switch resourceType {
	case ResourceTypeModule:
		modules, err := s.listModules(ctx)
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
		providers, err := s.listProviders(ctx)
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
func (s *S3Storage) Upload(ctx context.Context, id ResourceIdentifier, filepath string) error {
	switch id.Type {
	case ResourceTypeModule:
		return s.uploadModuleArchive(ctx, id.Namespace, id.Name, id.System, id.Version, filepath)
	case ResourceTypeProvider:
		return s.uploadProviderBinary(ctx, id.Namespace, id.Name, id.Version, id.OS, id.Arch, filepath)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// UpdateMetadata implements Writer.UpdateMetadata
func (s *S3Storage) UpdateMetadata(ctx context.Context, id ResourceIdentifier, metadata interface{}) error {
	switch id.Type {
	case ResourceTypeModule:
		moduleMeta, ok := metadata.(*models.ModuleMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for module: expected *models.ModuleMetadata")
		}
		return s.uploadModuleMetadata(ctx, id.Namespace, id.Name, id.System, moduleMeta)
	case ResourceTypeProvider:
		providerMeta, ok := metadata.(*models.ProviderMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for provider: expected *models.ProviderMetadata")
		}
		return s.uploadProviderMetadata(ctx, id.Namespace, id.Name, providerMeta)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// Delete implements Writer.Delete
func (s *S3Storage) Delete(ctx context.Context, id ResourceIdentifier) error {
	switch id.Type {
	case ResourceTypeModule:
		return s.deleteModule(ctx, id.Namespace, id.Name, id.System, id.Version)
	case ResourceTypeProvider:
		return s.deleteProvider(ctx, id.Namespace, id.Name, id.Version)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}
