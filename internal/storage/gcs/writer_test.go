package gcs

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/matryer/is"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

func TestUpload_Module_Success(t *testing.T) {
	is := is.New(t)

	var uploadedKeys []string
	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						newWriterFunc: func(ctx context.Context) Writer {
							uploadedKeys = append(uploadedKeys, objName)
							return &mockWriter{}
						},
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist // No existing metadata
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	testFile := createTestFile(t, "test module content")

	err := st.Upload(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Version:   "1.0.0",
	}, testFile)

	is.NoErr(err)
	is.True(len(uploadedKeys) >= 1) // At least archive uploaded
}

func TestUpload_Provider_Success(t *testing.T) {
	is := is.New(t)

	var uploadedKeys []string
	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						newWriterFunc: func(ctx context.Context) Writer {
							uploadedKeys = append(uploadedKeys, objName)
							return &mockWriter{}
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	testFile := createTestFile(t, "test provider binary content")

	err := st.Upload(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "myorg",
		Name:      "myprovider",
		Version:   "1.0.0",
		OS:        "linux",
		Arch:      "amd64",
	}, testFile)

	is.NoErr(err)
	is.Equal(len(uploadedKeys), 2) // Binary + platform metadata
}

func TestUpload_InvalidFile(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	err := st.Upload(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Version:   "1.0.0",
	}, "/nonexistent/file.zip")

	is.True(err != nil)
}

func TestUpload_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	err := st.Upload(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	}, "/some/file.zip")

	is.True(err != nil)
}

func TestDelete_Module_Success(t *testing.T) {
	is := is.New(t)

	var deletedKeys []string
	moduleMetadata := models.ModuleMetadata{
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Versions:  []models.Version{{Version: "1.0.0"}, {Version: "2.0.0"}},
	}
	metadataJSON, _ := json.Marshal(moduleMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						deleteFunc: func(ctx context.Context) error {
							deletedKeys = append(deletedKeys, objName)
							return nil
						},
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
						newWriterFunc: func(ctx context.Context) Writer {
							return &mockWriter{}
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	err := st.Delete(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Version:   "1.0.0",
	})

	is.NoErr(err)
	is.True(len(deletedKeys) >= 1) // Archive deleted
}

func TestDelete_Module_LastVersion(t *testing.T) {
	is := is.New(t)

	var deletedKeys []string
	moduleMetadata := models.ModuleMetadata{
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Versions:  []models.Version{{Version: "1.0.0"}}, // Only one version
	}
	metadataJSON, _ := json.Marshal(moduleMetadata)

	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						deleteFunc: func(ctx context.Context) error {
							deletedKeys = append(deletedKeys, objName)
							return nil
						},
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return &mockReader{Reader: bytes.NewReader(metadataJSON)}, nil
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	err := st.Delete(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Version:   "1.0.0",
	})

	is.NoErr(err)
	is.Equal(len(deletedKeys), 2) // Archive + metadata deleted
}

func TestDelete_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	err := st.Delete(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	})

	is.True(err != nil)
}

func TestUpdateMetadata_Module_Success(t *testing.T) {
	is := is.New(t)

	var uploadedMetadata []byte
	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist // No existing metadata
						},
						newWriterFunc: func(ctx context.Context) Writer {
							return &mockWriter{
								writeFunc: func(p []byte) (n int, err error) {
									uploadedMetadata = append(uploadedMetadata, p...)
									return len(p), nil
								},
							}
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	metadata := &models.ModuleMetadata{
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
		Versions:  []models.Version{{Version: "1.0.0"}},
	}

	err := st.UpdateMetadata(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
	}, metadata)

	is.NoErr(err)
	is.True(len(uploadedMetadata) > 0)
}

func TestUpdateMetadata_Provider_Success(t *testing.T) {
	is := is.New(t)

	var uploadedMetadata []byte
	mock := &mockGCSClient{
		bucketFunc: func(name string) BucketHandle {
			return &mockBucketHandle{
				objectFunc: func(objName string) ObjectHandle {
					return &mockObjectHandle{
						newReaderFunc: func(ctx context.Context) (Reader, error) {
							return nil, gcsstorage.ErrObjectNotExist // No existing metadata
						},
						newWriterFunc: func(ctx context.Context) Writer {
							return &mockWriter{
								writeFunc: func(p []byte) (n int, err error) {
									uploadedMetadata = append(uploadedMetadata, p...)
									return len(p), nil
								},
							}
						},
					}
				},
			}
		},
	}
	st := newTestStorage(mock)

	metadata := &models.ProviderMetadata{
		Namespace: "myorg",
		Type:      "myprovider",
		Versions:  []models.ProviderVersion{{Version: "1.0.0"}},
	}

	err := st.UpdateMetadata(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: "myorg",
		Name:      "myprovider",
	}, metadata)

	is.NoErr(err)
	is.True(len(uploadedMetadata) > 0)
}

func TestUpdateMetadata_InvalidType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	err := st.UpdateMetadata(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: "myorg",
		Name:      "mymodule",
		System:    "aws",
	}, "not a metadata struct")

	is.True(err != nil)
}

func TestUpdateMetadata_InvalidResourceType(t *testing.T) {
	is := is.New(t)
	mock := &mockGCSClient{}
	st := newTestStorage(mock)

	err := st.UpdateMetadata(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	}, &models.ModuleMetadata{})

	is.True(err != nil)
}

