package gcs

import (
	"testing"

	"github.com/matryer/is"
)

func TestGetModuleMetadataKey(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})

	key := st.getModuleMetadataKey("hashicorp", "consul", "aws")
	is.Equal(key, "registry/modules/hashicorp/consul/aws/metadata.json")
}

func TestGetModuleArchiveKey(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})

	key := st.getModuleArchiveKey("hashicorp", "consul", "aws", "1.0.0")
	is.Equal(key, "registry/modules/hashicorp/consul/aws/1.0.0/archive.tar.gz")
}

func TestGetProviderMetadataKey(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})

	key := st.getProviderMetadataKey("hashicorp", "aws")
	is.Equal(key, "registry/providers/hashicorp/aws/metadata.json")
}

func TestGetProviderBinaryMetadataKey(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})

	key := st.getProviderBinaryMetadataKey("hashicorp", "aws", "5.0.0", "linux", "amd64")
	is.Equal(key, "registry/providers/hashicorp/aws/5.0.0/linux/amd64/metadata.json")
}

func TestGetObjectURL_GCS(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})
	st.endpoint = "" // GCS mode

	url := st.getObjectURL("registry/providers/hashicorp/aws/5.0.0/file.zip")
	is.Equal(url, "https://storage.googleapis.com/test-bucket/registry/providers/hashicorp/aws/5.0.0/file.zip")
}

func TestGetObjectURL_FakeGCS(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})
	st.endpoint = "http://localhost:4443"

	url := st.getObjectURL("registry/providers/hashicorp/aws/5.0.0/file.zip")
	is.Equal(url, "http://localhost:4443/test-bucket/registry/providers/hashicorp/aws/5.0.0/file.zip")
}

func TestGetModuleMetadataKey_EmptyPrefix(t *testing.T) {
	is := is.New(t)
	st := newTestStorage(&mockGCSClient{})
	st.prefix = ""

	key := st.getModuleMetadataKey("test", "module", "aws")
	is.Equal(key, "/modules/test/module/aws/metadata.json")
}
