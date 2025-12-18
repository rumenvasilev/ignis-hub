package s3

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	registryConfig "github.com/rumenvasilev/ignis-hub/internal/config"
)

// S3Client defines the interface for S3 operations used by Storage.
// This allows for mocking in tests.
type S3Client interface {
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

// S3ListClient extends S3Client with listing capabilities.
// Separated because ListObjectsV2 is used via paginator.
type S3ListClient interface {
	S3Client
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// Compile-time check that *s3.Client satisfies S3ListClient.
var _ S3ListClient = (*s3.Client)(nil)

// Storage implements the storage.Storage interface for AWS S3.
type Storage struct {
	client   S3ListClient
	bucket   string
	prefix   string
	region   string
	endpoint string // For LocalStack/custom endpoints
	logger   *slog.Logger
}

// Unused import guard for types (used in interface but not directly here).
var _ = types.ChecksumAlgorithmSha256

// New creates a new S3 storage instance.
func New(ctx context.Context, cfg *registryConfig.Config, logger *slog.Logger) (*Storage, error) {
	// Build AWS SDK config options
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.AWS.Region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(_ context.Context) (aws.Credentials, error) {
			logger.Debug("Using explicit credentials")
			return aws.Credentials{
				AccessKeyID:     cfg.AWS.AccessKeyID,
				SecretAccessKey: cfg.AWS.SecretAccessKey,
				SessionToken:    cfg.AWS.SessionToken,
				Source:          "ExplicitCredentials",
			}, nil
		})),
	}

	// Add custom endpoint for LocalStack/local development
	if cfg.AWS.Endpoint != "" {
		configOpts = append(configOpts, config.WithBaseEndpoint(cfg.AWS.Endpoint))
		logger.Debug("Using custom AWS endpoint for local development")
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, configOpts...)
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

	return &Storage{
		client:   client,
		bucket:   cfg.AWS.S3Bucket,
		prefix:   cfg.AWS.S3Prefix,
		region:   cfg.AWS.Region,
		endpoint: cfg.AWS.Endpoint,
		logger:   logger,
	}, nil
}

// Key generation methods (private helpers)

func (s *Storage) getModuleMetadataKey(namespace, name, system string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/metadata.json", s.prefix, namespace, name, system)
}

func (s *Storage) getModuleArchiveKey(namespace, name, system, version string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/%s/archive.tar.gz", s.prefix, namespace, name, system, version)
}

func (s *Storage) getProviderMetadataKey(namespace, typeName string) string {
	return fmt.Sprintf("%s/providers/%s/%s/metadata.json", s.prefix, namespace, typeName)
}

func (s *Storage) getProviderBinaryMetadataKey(namespace, typeName, version, os, arch string) string {
	return fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/metadata.json", s.prefix, namespace, typeName, version, os, arch)
}

// getObjectURL returns the download URL for an S3 object.
// For LocalStack/custom endpoints, uses path-style URL.
// For AWS, uses virtual-hosted style URL.
func (s *Storage) getObjectURL(key string) string {
	if s.endpoint != "" {
		// LocalStack/custom endpoint: path-style URL
		return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	}
	// AWS: virtual-hosted style URL
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
}
