package s3

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// mockS3Client implements S3ListClient for testing.
type mockS3Client struct {
	headBucketFunc    func(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	headObjectFunc    func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	getObjectFunc     func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	putObjectFunc     func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	deleteObjectFunc  func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	deleteObjectsFunc func(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	listObjectsV2Func func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (m *mockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if m.headBucketFunc != nil {
		return m.headBucketFunc(ctx, params, optFns...)
	}
	return &s3.HeadBucketOutput{}, nil
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObjectFunc != nil {
		return m.headObjectFunc(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if m.deleteObjectFunc != nil {
		return m.deleteObjectFunc(ctx, params, optFns...)
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	if m.deleteObjectsFunc != nil {
		return m.deleteObjectsFunc(ctx, params, optFns...)
	}
	return &s3.DeleteObjectsOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsV2Func != nil {
		return m.listObjectsV2Func(ctx, params, optFns...)
	}
	return &s3.ListObjectsV2Output{}, nil
}

// Verify mockS3Client implements S3ListClient
var _ S3ListClient = (*mockS3Client)(nil)

// createTestFile is a helper to create a test file
func createTestFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-file.zip")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return filePath
}

