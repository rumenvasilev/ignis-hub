package s3

import (
	"io"
	"log/slog"
	"testing"

	"github.com/matryer/is"
)

func newTestStorage(mock *mockS3Client) *Storage {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Storage{
		client:   mock,
		bucket:   "test-bucket",
		prefix:   "registry",
		region:   "us-east-1",
		endpoint: "",
		logger:   logger,
	}
}

func TestGetModuleMetadataKey(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	key := st.getModuleMetadataKey("hashicorp", "consul", "aws")
	is.Equal(key, "registry/modules/hashicorp/consul/aws/metadata.json")
}

func TestGetModuleArchiveKey(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	key := st.getModuleArchiveKey("hashicorp", "consul", "aws", "1.0.0")
	is.Equal(key, "registry/modules/hashicorp/consul/aws/1.0.0/archive.tar.gz")
}

func TestGetProviderMetadataKey(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	key := st.getProviderMetadataKey("hashicorp", "aws")
	is.Equal(key, "registry/providers/hashicorp/aws/metadata.json")
}

func TestGetProviderBinaryMetadataKey(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	key := st.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	is.Equal(key, "registry/providers/hashicorp/aws/5.0.0/linux/amd64/metadata.json")
}

func TestGetObjectURL_AWS(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)
	st.endpoint = "" // AWS mode

	url := st.getObjectURL("registry/providers/hashicorp/aws/5.0.0/file.zip")
	is.Equal(url, "https://test-bucket.s3.us-east-1.amazonaws.com/registry/providers/hashicorp/aws/5.0.0/file.zip")
}

func TestGetObjectURL_LocalStack(t *testing.T) {
	is := is.New(t)
	mock := &mockS3Client{}
	st := newTestStorage(mock)
	st.endpoint = "http://localhost:4566"

	url := st.getObjectURL("registry/providers/hashicorp/aws/5.0.0/file.zip")
	is.Equal(url, "http://localhost:4566/test-bucket/registry/providers/hashicorp/aws/5.0.0/file.zip")
}

