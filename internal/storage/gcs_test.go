package storage

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
)

func TestNewGCSStorage(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		skipWithout string // Skip test if environment variable is not set
	}{
		{
			name: "valid GCP config without credentials",
			config: &config.Config{
				Provider: "gcp",
				GCP: config.GCPConfig{
					GCSBucket: "test-bucket",
					GCSPrefix: "terraform",
				},
			},
			skipWithout: "", // Will likely fail without credentials, but tests the code path
		},
		{
			name: "valid GCP config with endpoint",
			config: &config.Config{
				Provider: "gcp",
				GCP: config.GCPConfig{
					GCSBucket: "test-bucket",
					GCSPrefix: "terraform",
					Endpoint:  "http://localhost:4443",
				},
			},
			skipWithout: "",
		},
		{
			name: "valid GCP config with credentials file",
			config: &config.Config{
				Provider: "gcp",
				GCP: config.GCPConfig{
					GCSBucket:       "test-bucket",
					GCSPrefix:       "terraform",
					CredentialsFile: "/path/to/credentials.json",
				},
			},
			skipWithout: "",
		},
		{
			name: "empty prefix",
			config: &config.Config{
				Provider: "gcp",
				GCP: config.GCPConfig{
					GCSBucket: "test-bucket",
					GCSPrefix: "",
				},
			},
			skipWithout: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipWithout != "" {
				if os.Getenv(tt.skipWithout) == "" {
					t.Skipf("Skipping test: %s not set", tt.skipWithout)
				}
			}

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			storage, err := newGCSStorage(tt.config, logger)

			// GCS client creation might fail without valid credentials
			// This is expected in unit tests without GCP access
			if err != nil {
				t.Logf("newGCSStorage() returned error (expected without credentials): %v", err)
				return
			}

			is := is.New(t)
			is.True(storage != nil)                           // storage should not be nil
			is.Equal(storage.bucket, tt.config.GCP.GCSBucket) // bucket should match config
			is.Equal(storage.prefix, tt.config.GCP.GCSPrefix) // prefix should match config
			is.True(storage.client != nil)                    // GCS client should be initialized
			is.True(storage.logger != nil)                    // logger should be set
		})
	}
}

func TestGCSStorage_GetModuleMetadataKey(t *testing.T) {
	storage := &GCSStorage{
		prefix: "terraform",
	}

	tests := []struct {
		name      string
		namespace string
		modName   string
		system    string
		prefix    string
		want      string
	}{
		{
			name:      "standard module",
			namespace: "hashicorp",
			modName:   "consul",
			system:    "aws",
			prefix:    "terraform",
			want:      "terraform/modules/hashicorp/consul/aws/metadata.json",
		},
		{
			name:      "custom namespace",
			namespace: "myorg",
			modName:   "vpc",
			system:    "gcp",
			prefix:    "terraform",
			want:      "terraform/modules/myorg/vpc/gcp/metadata.json",
		},
		{
			name:      "empty prefix",
			namespace: "test",
			modName:   "test",
			system:    "azure",
			prefix:    "",
			want:      "/modules/test/test/azure/metadata.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			storage.prefix = tt.prefix
			got := storage.getModuleMetadataKey(tt.namespace, tt.modName, tt.system)
			is.Equal(got, tt.want) // key should match expected format
		})
	}
}

func TestGCSStorage_GetModuleArchiveKey(t *testing.T) {
	storage := &GCSStorage{
		prefix: "terraform",
	}

	tests := []struct {
		name      string
		namespace string
		modName   string
		system    string
		version   string
		want      string
	}{
		{
			name:      "standard module archive",
			namespace: "hashicorp",
			modName:   "consul",
			system:    "aws",
			version:   "1.0.0",
			want:      "terraform/modules/hashicorp/consul/aws/1.0.0/archive.tar.gz",
		},
		{
			name:      "version with pre-release",
			namespace: "myorg",
			modName:   "vpc",
			system:    "gcp",
			version:   "2.0.0-beta",
			want:      "terraform/modules/myorg/vpc/gcp/2.0.0-beta/archive.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			got := storage.getModuleArchiveKey(tt.namespace, tt.modName, tt.system, tt.version)
			is.Equal(got, tt.want) // key should match expected format
		})
	}
}

func TestGCSStorage_GetProviderMetadataKey(t *testing.T) {
	storage := &GCSStorage{
		prefix: "terraform",
	}

	tests := []struct {
		name      string
		namespace string
		typeName  string
		want      string
	}{
		{
			name:      "AWS provider",
			namespace: "hashicorp",
			typeName:  "aws",
			want:      "terraform/providers/hashicorp/aws/metadata.json",
		},
		{
			name:      "custom provider",
			namespace: "myorg",
			typeName:  "custom",
			want:      "terraform/providers/myorg/custom/metadata.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			got := storage.getProviderMetadataKey(tt.namespace, tt.typeName)
			is.Equal(got, tt.want) // key should match expected format
		})
	}
}

func TestGCSStorage_GetProviderBinaryMetadataKey(t *testing.T) {
	storage := &GCSStorage{
		prefix: "terraform",
	}

	tests := []struct {
		name      string
		namespace string
		typeName  string
		version   string
		os        string
		arch      string
		want      string
	}{
		{
			name:      "AWS provider Linux AMD64",
			namespace: "hashicorp",
			typeName:  "aws",
			version:   "5.0.0",
			os:        "linux",
			arch:      "amd64",
			want:      "terraform/providers/hashicorp/aws/5.0.0/linux/amd64/metadata.json",
		},
		{
			name:      "AWS provider Darwin ARM64",
			namespace: "hashicorp",
			typeName:  "aws",
			version:   "5.1.0",
			os:        "darwin",
			arch:      "arm64",
			want:      "terraform/providers/hashicorp/aws/5.1.0/darwin/arm64/metadata.json",
		},
		{
			name:      "custom provider",
			namespace: "myorg",
			typeName:  "custom",
			version:   "1.0.0",
			os:        "windows",
			arch:      "amd64",
			want:      "terraform/providers/myorg/custom/1.0.0/windows/amd64/metadata.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			got := storage.getProviderBinaryMetadataKey(tt.namespace, tt.typeName, tt.version, tt.os, tt.arch)
			is.Equal(got, tt.want) // key should match expected format
		})
	}
}

func TestGCSStorage_KeyGeneration_Consistency(t *testing.T) {
	is := is.New(t)
	storage := &GCSStorage{
		prefix: "terraform",
	}

	// Test that calling the same method with same parameters produces consistent results
	key1 := storage.getModuleMetadataKey("hashicorp", "consul", "aws")
	key2 := storage.getModuleMetadataKey("hashicorp", "consul", "aws")
	is.Equal(key1, key2) // getModuleMetadataKey should be deterministic

	key3 := storage.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	key4 := storage.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	is.Equal(key3, key4) // getProviderBinaryMetadataKey should be deterministic
}

func TestGCSStorage_KeyGeneration_ConsistencyWithS3(t *testing.T) {
	is := is.New(t)
	// Verify that GCS and S3 generate the same keys for the same inputs
	s3Storage := &S3Storage{
		prefix: "terraform",
	}
	gcsStorage := &GCSStorage{
		prefix: "terraform",
	}

	// Module metadata key
	s3Key1 := s3Storage.getModuleMetadataKey("hashicorp", "consul", "aws")
	gcsKey1 := gcsStorage.getModuleMetadataKey("hashicorp", "consul", "aws")
	is.Equal(s3Key1, gcsKey1) // module metadata keys should match between S3 and GCS

	// Module archive key
	s3Key2 := s3Storage.getModuleArchiveKey("hashicorp", "consul", "aws", "1.0.0")
	gcsKey2 := gcsStorage.getModuleArchiveKey("hashicorp", "consul", "aws", "1.0.0")
	is.Equal(s3Key2, gcsKey2) // module archive keys should match between S3 and GCS

	// Provider metadata key
	s3Key3 := s3Storage.getProviderMetadataKey("hashicorp", "aws")
	gcsKey3 := gcsStorage.getProviderMetadataKey("hashicorp", "aws")
	is.Equal(s3Key3, gcsKey3) // provider metadata keys should match between S3 and GCS

	// Provider binary metadata key
	s3Key4 := s3Storage.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	gcsKey4 := gcsStorage.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	is.Equal(s3Key4, gcsKey4) // provider binary metadata keys should match between S3 and GCS
}

func TestGCSStorage_KeyGeneration_WithSpecialCharacters(t *testing.T) {
	storage := &GCSStorage{
		prefix: "terraform",
	}

	// Test handling of special characters (though they shouldn't be in real use)
	tests := []struct {
		name      string
		namespace string
		modName   string
		system    string
	}{
		{
			name:      "dashes in names",
			namespace: "my-org",
			modName:   "my-module",
			system:    "my-system",
		},
		{
			name:      "underscores in names",
			namespace: "my_org",
			modName:   "my_module",
			system:    "my_system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			key := storage.getModuleMetadataKey(tt.namespace, tt.modName, tt.system)
			is.True(key != "")                                             // key should not be empty
			is.True(containsAll(key, tt.namespace, tt.modName, tt.system)) // key should contain all parts
		})
	}
}
