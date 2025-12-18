package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/matryer/is"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

func TestUpload_Module_Success(t *testing.T) {
	is := is.New(t)

	var uploadedKeys []string
	mock := &mockS3Client{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			uploadedKeys = append(uploadedKeys, *params.Key)
			return &s3.PutObjectOutput{}, nil
		},
		getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, &types.NoSuchKey{} // No existing metadata
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
	mock := &mockS3Client{
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			uploadedKeys = append(uploadedKeys, *params.Key)
			return &s3.PutObjectOutput{}, nil
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
	mock := &mockS3Client{}
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
	mock := &mockS3Client{}
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

	mock := &mockS3Client{
		deleteObjectFunc: func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			deletedKeys = append(deletedKeys, *params.Key)
			return &s3.DeleteObjectOutput{}, nil
		},
		getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader(metadataJSON)),
			}, nil
		},
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return &s3.PutObjectOutput{}, nil
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

	mock := &mockS3Client{
		deleteObjectFunc: func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			deletedKeys = append(deletedKeys, *params.Key)
			return &s3.DeleteObjectOutput{}, nil
		},
		getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader(metadataJSON)),
			}, nil
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
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	err := st.Delete(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	})

	is.True(err != nil)
}

func TestUpdateMetadata_Module_Success(t *testing.T) {
	is := is.New(t)

	var uploadedMetadata []byte
	mock := &mockS3Client{
		getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, &types.NoSuchKey{} // No existing metadata
		},
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			body, _ := io.ReadAll(params.Body)
			uploadedMetadata = body
			return &s3.PutObjectOutput{}, nil
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
	mock := &mockS3Client{
		getObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, &types.NoSuchKey{} // No existing metadata
		},
		putObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			body, _ := io.ReadAll(params.Body)
			uploadedMetadata = body
			return &s3.PutObjectOutput{}, nil
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
	mock := &mockS3Client{}
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
	mock := &mockS3Client{}
	st := newTestStorage(mock)

	err := st.UpdateMetadata(context.Background(), api.ResourceIdentifier{
		Type: "invalid",
	}, &models.ModuleMetadata{})

	is.True(err != nil)
}

