package gcs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/matryer/is"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

func TestHealthCheck_Success(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			is.Equal(name, "test-bucket")
			return &mockBucketHandle{
				attrsFunc: func(ctx context.Context) (*gcsstorage.BucketAttrs, error) {
					return &gcsstorage.BucketAttrs{}, nil
				},
			}
		},
	}
	st := newTestStorage(mock)

	err := st.HealthCheck(context.Background())
	is.NoErr(err)
}

func TestHealthCheck_Failure(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				attrsFunc: func(ctx context.Context) (*gcsstorage.BucketAttrs, error) {
					return nil, errors.New("bucket not found")
				},
			}
		},
	}
	st := newTestStorage(mock)

	err := st.HealthCheck(context.Background())
	is.True(err != nil)
}

func TestGetVersions_Module_Success(t *testing.T) {
	is := is.New(t)

	moduleMetadata := models.ModuleMetadata{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions:  []models.Version{{Version: "1.0.0"}, {Version: "1.1.0"}},
	}
	metadataJSON, _ := json.Marshal(moduleMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
	})

	is.NoErr(err)
	is.True(resp != nil)
	is.True(resp.Module != nil)
	is.Equal(resp.Module.Namespace, "hashicorp")
	is.Equal(len(resp.Module.Versions), 2)
}

func TestGetVersions_Module_NotFound(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "hashicorp",
		Name:      "nonexistent",
		System:    "aws",
	})

	is.True(err != nil)
	is.True(errors.Is(err, api.ErrModuleNotFound))
	is.True(resp == nil)
}

func TestGetVersions_Provider_Success(t *testing.T) {
	is := is.New(t)

	providerMetadata := models.ProviderMetadata{
		Namespace: "hashicorp",
		Type:      "aws",
		Versions: []models.ProviderVersion{
			{Version: "5.0.0", Platforms: []models.Platform{{OS: "linux", Arch: "amd64"}}},
		},
	}
	metadataJSON, _ := json.Marshal(providerMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "hashicorp",
		Name:      "aws",
	})

	is.NoErr(err)
	is.True(resp != nil)
	is.True(resp.Provider != nil)
	is.Equal(resp.Provider.Namespace, "hashicorp")
	is.Equal(resp.Provider.Type, "aws")
}

func TestGetVersions_Provider_NotFound(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "hashicorp",
		Name:      "nonexistent",
	})

	is.True(err != nil)
	is.True(errors.Is(err, api.ErrProviderNotFound))
	is.True(resp == nil)
}

func TestGetVersions_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	resp, err := st.GetVersions(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	})

	is.True(err != nil)
	is.True(resp == nil)
}

func TestGetDownloadInfo_Module_Success(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						attrsFunc: func(ctx context.Context) (*gcsstorage.ObjectAttrs, error) {
							return &gcsstorage.ObjectAttrs{}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetDownloadInfo(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Version:   "1.0.0",
	})

	is.NoErr(err)
	is.True(resp != nil)
	is.True(resp.URL != "")
}

func TestGetDownloadInfo_Module_NotFound(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						attrsFunc: func(ctx context.Context) (*gcsstorage.ObjectAttrs, error) {
							return nil, gcsstorage.ErrObjectNotExist
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetDownloadInfo(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Version:   "9.9.9",
	})

	is.True(err != nil)
	is.True(errors.Is(err, api.ErrModuleVersionNotFound))
	is.True(resp == nil)
}

func TestGetDownloadInfo_Provider_Success(t *testing.T) {
	is := is.New(t)

	binaryMetadata := models.ProviderBinaryMetadata{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://example.com/download",
		Shasum:      "abc123",
	}
	metadataJSON, _ := json.Marshal(binaryMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetDownloadInfo(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "hashicorp",
		Name:      "aws",
		Version:   "5.0.0",
		OS:        "linux",
		Arch:      "amd64",
	})

	is.NoErr(err)
	is.True(resp != nil)
	is.True(resp.ProviderBinary != nil)
	is.Equal(resp.ProviderBinary.Filename, "terraform-provider-aws_5.0.0_linux_amd64.zip")
}

func TestGetDownloadInfo_Provider_NotFound(t *testing.T) {
	is := is.New(t)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resp, err := st.GetDownloadInfo(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "hashicorp",
		Name:      "nonexistent",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
	})

	is.True(err != nil)
	is.True(errors.Is(err, api.ErrProviderBinaryNotFound))
	is.True(resp == nil)
}

func TestGetDownloadInfo_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	resp, err := st.GetDownloadInfo(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	})

	is.True(err != nil)
	is.True(resp == nil)
}

func TestList_Module_Success(t *testing.T) {
	is := is.New(t)

	moduleMetadata := models.ModuleMetadata{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions:  []models.Version{{Version: "1.0.0"}},
	}
	metadataJSON, _ := json.Marshal(moduleMetadata)

	// Track which objects are being requested
	objectCallCount := 0

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectsFunc: func(ctx context.Context, q *gcsstorage.Query) ObjectIterator {
					// Return different iterators based on the query
					if q.Delimiter == "/" {
						// Namespace/name/system listing
						switch {
						case q.Prefix == "registry/modules/":
							return &mockObjectIterator{
								objects: []*gcsstorage.ObjectAttrs{
									{Prefix: "registry/modules/hashicorp/"},
								},
							}
						case q.Prefix == "registry/modules/hashicorp/":
							return &mockObjectIterator{
								objects: []*gcsstorage.ObjectAttrs{
									{Prefix: "registry/modules/hashicorp/consul/"},
								},
							}
						case q.Prefix == "registry/modules/hashicorp/consul/":
							return &mockObjectIterator{
								objects: []*gcsstorage.ObjectAttrs{
									{Prefix: "registry/modules/hashicorp/consul/aws/"},
								},
							}
						}
					}
					return &mockObjectIterator{}
				},
				objectFunc: func(name string) ObjectHandle {
					objectCallCount++
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resources, err := st.List(context.Background(), api.ResourceTypeModule)

	is.NoErr(err)
	is.Equal(len(resources), 1)
	is.Equal(resources[0].Namespace, "hashicorp")
	is.Equal(resources[0].Name, "consul")
}

func TestList_Provider_Success(t *testing.T) {
	is := is.New(t)

	providerMetadata := models.ProviderMetadata{
		Namespace: "hashicorp",
		Type:      "aws",
		Versions:  []models.ProviderVersion{{Version: "5.0.0"}},
	}
	metadataJSON, _ := json.Marshal(providerMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectsFunc: func(ctx context.Context, q *gcsstorage.Query) ObjectIterator {
					// Return different iterators based on the query
					if q.Delimiter == "/" {
						switch {
						case q.Prefix == "registry/providers/":
							return &mockObjectIterator{
								objects: []*gcsstorage.ObjectAttrs{
									{Prefix: "registry/providers/hashicorp/"},
								},
							}
						case q.Prefix == "registry/providers/hashicorp/":
							return &mockObjectIterator{
								objects: []*gcsstorage.ObjectAttrs{
									{Prefix: "registry/providers/hashicorp/aws/"},
								},
							}
						}
					}
					return &mockObjectIterator{}
				},
				objectFunc: func(name string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	resources, err := st.List(context.Background(), api.ResourceTypeProvider)

	is.NoErr(err)
	is.Equal(len(resources), 1)
	is.Equal(resources[0].Namespace, "hashicorp")
	is.Equal(resources[0].Name, "aws")
}

func TestList_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	resources, err := st.List(context.Background(), "invalid")

	is.True(err != nil)
	is.True(resources == nil)
}

