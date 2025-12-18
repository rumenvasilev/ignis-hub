package gcs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	gcsstorage "cloud.google.com/go/storage"
)

// mockGCSClient implements GCSClient for testing.
type mockGCSClient struct {
	bucketFunc func(name string) BucketHandle
}

func (m *mockGCSClient) Bucket(name string) BucketHandle {
	if m.bucketFunc != nil {
		return m.bucketFunc(name)
	}
	return &mockBucketHandle{}
}

// mockBucketHandle implements BucketHandle for testing.
type mockBucketHandle struct {
	objectFunc  func(name string) ObjectHandle
	objectsFunc func(ctx context.Context, q *gcsstorage.Query) ObjectIterator
	attrsFunc   func(ctx context.Context) (*gcsstorage.BucketAttrs, error)
}

func (m *mockBucketHandle) Object(name string) ObjectHandle {
	if m.objectFunc != nil {
		return m.objectFunc(name)
	}
	return &mockObjectHandle{}
}

func (m *mockBucketHandle) Objects(ctx context.Context, q *gcsstorage.Query) ObjectIterator {
	if m.objectsFunc != nil {
		return m.objectsFunc(ctx, q)
	}
	return &mockObjectIterator{}
}

func (m *mockBucketHandle) Attrs(ctx context.Context) (*gcsstorage.BucketAttrs, error) {
	if m.attrsFunc != nil {
		return m.attrsFunc(ctx)
	}
	return &gcsstorage.BucketAttrs{}, nil
}

// mockObjectHandle implements ObjectHandle for testing.
type mockObjectHandle struct {
	newReaderFunc func(ctx context.Context) (Reader, error)
	newWriterFunc func(ctx context.Context) Writer
	attrsFunc     func(ctx context.Context) (*gcsstorage.ObjectAttrs, error)
	deleteFunc    func(ctx context.Context) error
}

func (m *mockObjectHandle) NewReader(ctx context.Context) (Reader, error) {
	if m.newReaderFunc != nil {
		return m.newReaderFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockObjectHandle) NewWriter(ctx context.Context) Writer {
	if m.newWriterFunc != nil {
		return m.newWriterFunc(ctx)
	}
	return &mockWriter{}
}

func (m *mockObjectHandle) Attrs(ctx context.Context) (*gcsstorage.ObjectAttrs, error) {
	if m.attrsFunc != nil {
		return m.attrsFunc(ctx)
	}
	return &gcsstorage.ObjectAttrs{}, nil
}

func (m *mockObjectHandle) Delete(ctx context.Context) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx)
	}
	return nil
}

// mockWriter implements Writer for testing.
type mockWriter struct {
	writeFunc      func(p []byte) (n int, err error)
	closeFunc      func() error
	contentType    string
	md5            []byte
	writtenContent []byte
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	if m.writeFunc != nil {
		return m.writeFunc(p)
	}
	m.writtenContent = append(m.writtenContent, p...)
	return len(p), nil
}

func (m *mockWriter) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockWriter) SetContentType(contentType string) {
	m.contentType = contentType
}

func (m *mockWriter) SetMD5(md5 []byte) {
	m.md5 = md5
}

// mockObjectIterator implements ObjectIterator for testing.
type mockObjectIterator struct {
	objects []*gcsstorage.ObjectAttrs
	index   int
}

func (m *mockObjectIterator) Next() (*gcsstorage.ObjectAttrs, error) {
	if m.index >= len(m.objects) {
		return nil, iteratorDone
	}
	obj := m.objects[m.index]
	m.index++
	return obj, nil
}

// mockReader implements Reader for testing.
type mockReader struct {
	io.Reader
	closeFunc func() error
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	return m.Reader.Read(p)
}

func (m *mockReader) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// Verify mocks implement interfaces
var (
	_ GCSClient      = (*mockGCSClient)(nil)
	_ BucketHandle   = (*mockBucketHandle)(nil)
	_ ObjectHandle   = (*mockObjectHandle)(nil)
	_ Writer         = (*mockWriter)(nil)
	_ ObjectIterator = (*mockObjectIterator)(nil)
	_ Reader         = (*mockReader)(nil)
)

func newTestStorage(mock GCSClient) *Storage {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Storage{
		client:   mock,
		bucket:   "test-bucket",
		prefix:   "registry",
		endpoint: "",
		logger:   logger,
	}
}

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

