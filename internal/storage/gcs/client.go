package gcs

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	gcsstorage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	registryConfig "github.com/rumenvasilev/ignis-hub/internal/config"
)

// ObjectIterator defines the interface for iterating over GCS objects.
type ObjectIterator interface {
	Next() (*gcsstorage.ObjectAttrs, error)
}

// Writer defines the interface for writing to GCS.
type Writer interface {
	io.WriteCloser
	SetContentType(contentType string)
	SetMD5(md5 []byte)
}

// Reader defines the interface for reading from GCS.
type Reader interface {
	io.ReadCloser
}

// ObjectHandle defines the interface for GCS object operations.
type ObjectHandle interface {
	NewReader(ctx context.Context) (Reader, error)
	NewWriter(ctx context.Context) Writer
	Attrs(ctx context.Context) (*gcsstorage.ObjectAttrs, error)
	Delete(ctx context.Context) error
}

// BucketHandle defines the interface for GCS bucket operations.
type BucketHandle interface {
	Object(name string) ObjectHandle
	Objects(ctx context.Context, q *gcsstorage.Query) ObjectIterator
	Attrs(ctx context.Context) (*gcsstorage.BucketAttrs, error)
}

// GCSClient defines the interface for the GCS client.
type GCSClient interface {
	Bucket(name string) BucketHandle
}

// Adapters to wrap the real GCS types to satisfy our interfaces

type gcsClientAdapter struct {
	client *gcsstorage.Client
}

func (a *gcsClientAdapter) Bucket(name string) BucketHandle {
	return &gcsBucketAdapter{bucket: a.client.Bucket(name)}
}

type gcsBucketAdapter struct {
	bucket *gcsstorage.BucketHandle
}

func (a *gcsBucketAdapter) Object(name string) ObjectHandle {
	return &gcsObjectAdapter{obj: a.bucket.Object(name)}
}

func (a *gcsBucketAdapter) Objects(ctx context.Context, q *gcsstorage.Query) ObjectIterator {
	return a.bucket.Objects(ctx, q)
}

func (a *gcsBucketAdapter) Attrs(ctx context.Context) (*gcsstorage.BucketAttrs, error) {
	return a.bucket.Attrs(ctx)
}

type gcsObjectAdapter struct {
	obj *gcsstorage.ObjectHandle
}

func (a *gcsObjectAdapter) NewReader(ctx context.Context) (Reader, error) {
	return a.obj.NewReader(ctx)
}

func (a *gcsObjectAdapter) NewWriter(ctx context.Context) Writer {
	return &gcsWriterAdapter{writer: a.obj.NewWriter(ctx)}
}

func (a *gcsObjectAdapter) Attrs(ctx context.Context) (*gcsstorage.ObjectAttrs, error) {
	return a.obj.Attrs(ctx)
}

func (a *gcsObjectAdapter) Delete(ctx context.Context) error {
	return a.obj.Delete(ctx)
}

type gcsWriterAdapter struct {
	writer *gcsstorage.Writer
}

func (a *gcsWriterAdapter) Write(p []byte) (n int, err error) {
	return a.writer.Write(p)
}

func (a *gcsWriterAdapter) Close() error {
	return a.writer.Close()
}

func (a *gcsWriterAdapter) SetContentType(contentType string) {
	a.writer.ContentType = contentType
}

func (a *gcsWriterAdapter) SetMD5(md5 []byte) {
	a.writer.MD5 = md5
}

// Compile-time check that iterator.Done is accessible
var _ = iterator.Done

// Storage implements the storage.Storage interface for Google Cloud Storage.
type Storage struct {
	client   GCSClient
	bucket   string
	prefix   string
	endpoint string // For local development/custom endpoints
	logger   *slog.Logger
}

// New creates a new GCS storage instance.
func New(ctx context.Context, cfg *registryConfig.Config, logger *slog.Logger) (*Storage, error) {
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

	client, err := gcsstorage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &Storage{
		client:   &gcsClientAdapter{client: client},
		bucket:   cfg.GCP.GCSBucket,
		prefix:   cfg.GCP.GCSPrefix,
		endpoint: cfg.GCP.Endpoint,
		logger:   logger,
	}, nil
}

// Key generation methods (private helpers)

func (st *Storage) getModuleMetadataKey(namespace, name, system string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/metadata.json", st.prefix, namespace, name, system)
}

func (st *Storage) getModuleArchiveKey(namespace, name, system, version string) string {
	return fmt.Sprintf("%s/modules/%s/%s/%s/%s/archive.tar.gz", st.prefix, namespace, name, system, version)
}

func (st *Storage) getProviderMetadataKey(namespace, typeName string) string {
	return fmt.Sprintf("%s/providers/%s/%s/metadata.json", st.prefix, namespace, typeName)
}

func (st *Storage) getProviderBinaryMetadataKey(namespace, typeName, version, os, arch string) string {
	return fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/metadata.json", st.prefix, namespace, typeName, version, os, arch)
}

// getObjectURL returns the download URL for a GCS object.
// For custom endpoints (local development), uses the endpoint URL.
// For GCS, uses the standard storage URL.
func (st *Storage) getObjectURL(key string) string {
	if st.endpoint != "" {
		// Custom endpoint (e.g., fake-gcs-server)
		return fmt.Sprintf("%s/%s/%s", st.endpoint, st.bucket, key)
	}
	// Standard GCS URL
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", st.bucket, key)
}
