package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

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

func newS3Storage(cfg *registryConfig.Config, logger *slog.Logger) (*S3Storage, error) {
	// Configure AWS SDK with explicit credentials first
	var awsConfig aws.Config
	var err error

	if cfg.AWS.Endpoint != "" {
		// For local development with LocalStack
		awsConfig, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.AWS.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
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
		awsConfig, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.AWS.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
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

// Module methods
func (s *S3Storage) GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
	key := s.getModuleMetadataKey(namespace, name, system)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("Failed to get module metadata", "error", err, "key", key)
		return nil, fmt.Errorf("module not found: %w", err)
	}
	defer result.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read module metadata: %w", err)
	}

	var metadata models.ModuleMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal module metadata: %w", err)
	}

	return &metadata, nil
}

func (s *S3Storage) GetModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	// For modules, we return a git URL based on naming convention
	// This assumes modules are stored as git repositories
	// You might want to customize this based on your storage strategy

	key := s.getModuleArchiveKey(namespace, name, system, version)

	// Check if the module version exists
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("Module version not found",
			"error", err,
			"namespace", namespace,
			"name", name,
			"system", system,
			"version", version)
		return "", fmt.Errorf("module version not found: %w", err)
	}

	// Generate a presigned URL for download
	presignClient := s3.NewPresignClient(s.client)
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// Provider methods
func (s *S3Storage) GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
	key := s.getProviderMetadataKey(namespace, typeName)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("Failed to get provider metadata", "error", err, "key", key)
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	defer result.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider metadata: %w", err)
	}

	var metadata models.ProviderMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider metadata: %w", err)
	}

	return &metadata, nil
}

func (s *S3Storage) GetProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
	key := s.getProviderBinaryMetadataKey(namespace, typeName, version, os, arch)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("Failed to get provider binary metadata", "error", err, "key", key)
		return nil, fmt.Errorf("provider binary not found: %w", err)
	}
	defer result.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(result.Body)
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

// Utility methods
func (s *S3Storage) GenerateDownloadURL(ctx context.Context, key string) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
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
