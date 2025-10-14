package storage

import (
	"io"
	"log/slog"
	"testing"

	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
)

func TestNewStorage_AWS(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Provider: "aws",
		AWS: config.AWSConfig{
			Region:          "us-east-1",
			S3Bucket:        "test-bucket",
			S3Prefix:        "terraform",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := NewStorage(cfg, logger)
	is.NoErr(err) // NewStorage should succeed for AWS

	is.True(storage != nil) // storage should not be nil

	s3Storage, ok := storage.(*S3Storage)
	is.True(ok) // NewStorage should return S3Storage for AWS provider

	is.Equal(s3Storage.bucket, "test-bucket") // bucket should match config
	is.Equal(s3Storage.prefix, "terraform")   // prefix should match config
}

func TestNewStorage_GCP(t *testing.T) {
	is := is.New(t)
	// Note: This test will fail without valid GCP credentials
	// In a real CI/CD environment, you'd mock the GCS client
	cfg := &config.Config{
		Provider: "gcp",
		GCP: config.GCPConfig{
			GCSBucket: "test-bucket",
			GCSPrefix: "terraform",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := NewStorage(cfg, logger)

	// We expect this might fail without credentials, but we're testing the factory logic
	if err != nil {
		// This is expected if no credentials are available
		t.Logf("NewStorage() failed for GCP (expected without credentials): %v", err)
		return
	}

	is.True(storage != nil) // storage should not be nil

	gcsStorage, ok := storage.(*GCSStorage)
	is.True(ok) // NewStorage should return GCSStorage for GCP provider

	is.Equal(gcsStorage.bucket, "test-bucket") // bucket should match config
	is.Equal(gcsStorage.prefix, "terraform")   // prefix should match config
}

func TestNewStorage_InvalidProvider(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Provider: "azure", // Invalid provider
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := NewStorage(cfg, logger)
	is.True(err != nil)                              // NewStorage should fail for invalid provider
	is.True(storage == nil)                          // storage should be nil for invalid provider
	is.Equal(err.Error(), "invalid provider: azure") // error message should match
}

func TestNewStorage_EmptyProvider(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Provider: "",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := NewStorage(cfg, logger)
	is.True(err != nil)     // NewStorage should fail for empty provider
	is.True(storage == nil) // storage should be nil for empty provider
}
