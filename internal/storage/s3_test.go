package storage

import (
	"io"
	"log/slog"
	"testing"

	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
)

func TestNewS3Storage(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		wantError bool
	}{
		{
			name: "valid AWS config",
			config: &config.Config{
				Provider: "aws",
				AWS: config.AWSConfig{
					Region:          "us-east-1",
					S3Bucket:        "test-bucket",
					S3Prefix:        "terraform",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
				},
			},
			wantError: false,
		},
		{
			name: "valid AWS config with endpoint (LocalStack)",
			config: &config.Config{
				Provider: "aws",
				AWS: config.AWSConfig{
					Region:          "us-east-1",
					S3Bucket:        "test-bucket",
					S3Prefix:        "terraform",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					Endpoint:        "http://localhost:4566",
				},
			},
			wantError: false,
		},
		{
			name: "empty prefix",
			config: &config.Config{
				Provider: "aws",
				AWS: config.AWSConfig{
					Region:          "us-east-1",
					S3Bucket:        "test-bucket",
					S3Prefix:        "",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			storage, err := newS3Storage(tt.config, logger)

			if tt.wantError {
				is.True(err != nil) // newS3Storage should return error
			} else {
				is.NoErr(err)                                    // newS3Storage should succeed
				is.True(storage != nil)                          // storage should not be nil
				is.Equal(storage.bucket, tt.config.AWS.S3Bucket) // bucket should match config
				is.Equal(storage.prefix, tt.config.AWS.S3Prefix) // prefix should match config
				is.True(storage.client != nil)                   // S3 client should be initialized
				is.True(storage.logger != nil)                   // logger should be set
			}
		})
	}
}

func TestS3Storage_GetModuleMetadataKey(t *testing.T) {
	is := is.New(t)
	storage := &S3Storage{
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

func TestS3Storage_GetModuleArchiveKey(t *testing.T) {
	is := is.New(t)
	storage := &S3Storage{
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

func TestS3Storage_GetProviderMetadataKey(t *testing.T) {
	is := is.New(t)
	storage := &S3Storage{
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

func TestS3Storage_GetProviderBinaryMetadataKey(t *testing.T) {
	is := is.New(t)
	storage := &S3Storage{
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

func TestS3Storage_KeyGeneration_Consistency(t *testing.T) {
	is := is.New(t)
	storage := &S3Storage{
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

func TestS3Storage_KeyGeneration_WithSpecialCharacters(t *testing.T) {
	storage := &S3Storage{
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

// Helper function to check if a string contains all substrings
func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		found := false
		for i := 0; i <= len(s)-len(part); i++ {
			if s[i:i+len(part)] == part {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
