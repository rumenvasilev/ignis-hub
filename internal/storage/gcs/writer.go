package gcs

import (
	"context"
	"crypto/md5" // #nosec G501 - MD5 is required by GCS for upload integrity validation
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	gcsstorage "cloud.google.com/go/storage"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

// Ensure gcsstorage.ErrObjectNotExist is accessible for type checking
var errObjectNotExist = gcsstorage.ErrObjectNotExist

// writeJSONToStorage marshals data to JSON and uploads it to GCS with MD5 checksum validation.
// This is the single point of truth for writing JSON metadata to storage.
func (st *Storage) writeJSONToStorage(ctx context.Context, key string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	hash := md5.Sum(jsonData) // #nosec G401 - MD5 required by GCS for integrity

	bucket := st.client.Bucket(st.bucket)
	obj := bucket.Object(key)
	writer := obj.NewWriter(ctx)
	writer.SetContentType("application/json")
	writer.SetMD5(hash[:])

	if _, err := writer.Write(jsonData); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to write JSON to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return nil
}

// Upload implements api.Writer.Upload.
func (st *Storage) Upload(ctx context.Context, id api.ResourceIdentifier, filePath string) error {
	switch id.Type {
	case api.ResourceTypeModule:
		return st.uploadModuleArchive(ctx, id.Namespace, id.Name, id.System, id.Version, filePath)
	case api.ResourceTypeProvider:
		return st.uploadProviderBinary(ctx, id.Namespace, id.Name, id.Version, id.OS, id.Arch, filePath)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// UpdateMetadata implements api.Writer.UpdateMetadata.
func (st *Storage) UpdateMetadata(ctx context.Context, id api.ResourceIdentifier, metadata interface{}) error {
	switch id.Type {
	case api.ResourceTypeModule:
		moduleMeta, ok := metadata.(*models.ModuleMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for module: expected *models.ModuleMetadata")
		}
		return st.uploadModuleMetadata(ctx, id.Namespace, id.Name, id.System, moduleMeta)
	case api.ResourceTypeProvider:
		providerMeta, ok := metadata.(*models.ProviderMetadata)
		if !ok {
			return fmt.Errorf("invalid metadata type for provider: expected *models.ProviderMetadata")
		}
		return st.uploadProviderMetadata(ctx, id.Namespace, id.Name, providerMeta)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// Delete implements api.Writer.Delete.
func (st *Storage) Delete(ctx context.Context, id api.ResourceIdentifier) error {
	switch id.Type {
	case api.ResourceTypeModule:
		return st.deleteModule(ctx, id.Namespace, id.Name, id.System, id.Version)
	case api.ResourceTypeProvider:
		return st.deleteProvider(ctx, id.Namespace, id.Name, id.Version)
	default:
		return fmt.Errorf("unknown resource type: %s", id.Type)
	}
}

// uploadProviderFile uploads the binary to GCS and returns metadata needed for the rest of the process.
// The file is closed when this function returns.
// path is the local filesystem path to the provider binary file.
func (st *Storage) uploadProviderFile(ctx context.Context, namespace, typeName, version, osName, arch, path string) (filename, key, shasum string, err error) {
	file, err := os.Open(filepath.Clean(path)) // #nosec G304 - path from trusted CLI input
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to stat file: %w", err)
	}

	filename = fileInfo.Name()
	key = fmt.Sprintf("%s/providers/%s/%s/%s/%s/%s/%s", st.prefix, namespace, typeName, version, osName, arch, filename)

	// Calculate SHA256 and MD5 in one pass (SHA256 for metadata, MD5 for GCS integrity)
	sha256Hash := sha256.New()
	md5Hash := md5.New() // #nosec G401 - MD5 required by GCS for integrity
	multiWriter := io.MultiWriter(sha256Hash, md5Hash)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return "", "", "", fmt.Errorf("failed to calculate checksums: %w", err)
	}
	shasum = hex.EncodeToString(sha256Hash.Sum(nil))
	md5sum := md5Hash.Sum(nil)

	// Seek back to beginning for upload
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", "", fmt.Errorf("failed to seek file: %w", err)
	}

	// Upload to GCS with MD5 checksum for data integrity
	writer := st.client.Bucket(st.bucket).Object(key).NewWriter(ctx)
	writer.SetMD5(md5sum)

	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return "", "", "", fmt.Errorf("failed to upload to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", "", "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	st.logger.Info("Uploaded provider binary", "key", key, "size", fileInfo.Size())

	return filename, key, shasum, nil
}

// uploadProviderBinary uploads a provider binary to api.
func (st *Storage) uploadProviderBinary(ctx context.Context, namespace, typeName, version, osName, arch, filePath string) error {
	// Upload file and compute hash - file is closed when this returns
	filename, key, shasum, err := st.uploadProviderFile(ctx, namespace, typeName, version, osName, arch, filePath)
	if err != nil {
		return err
	}

	// Generate download URL (standard GCS URL for VPC-internal access)
	downloadURL := st.getObjectURL(key)

	// Create platform metadata
	platformMetadata := &models.ProviderBinaryMetadata{
		Namespace:   namespace,
		Type:        typeName,
		Version:     version,
		OS:          osName,
		Arch:        arch,
		Filename:    filename,
		DownloadURL: downloadURL,
		Shasum:      shasum,
		Protocols:   []string{"5.0"},
	}

	// Marshal to JSON
	platformMetadataJSON, err := json.Marshal(platformMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal platform metadata: %w", err)
	}

	// Calculate MD5 checksum for integrity validation
	metadataMD5 := md5.Sum(platformMetadataJSON) // #nosec G401 - MD5 required by GCS

	// Upload platform metadata with MD5 checksum for data integrity
	platformMetadataKey := st.getProviderBinaryMetadataKey(namespace, typeName, version, osName, arch)
	metadataWriter := st.client.Bucket(st.bucket).Object(platformMetadataKey).NewWriter(ctx)
	metadataWriter.SetContentType("application/json")
	metadataWriter.SetMD5(metadataMD5[:])

	if _, err := metadataWriter.Write(platformMetadataJSON); err != nil {
		_ = metadataWriter.Close()
		return fmt.Errorf("failed to upload platform metadata: %w", err)
	}

	if err := metadataWriter.Close(); err != nil {
		return fmt.Errorf("failed to close metadata writer: %w", err)
	}

	st.logger.Info("Uploaded provider platform metadata", "key", platformMetadataKey)

	// After successful upload, update versions metadata
	metadata := &models.ProviderMetadata{
		Namespace: namespace,
		Type:      typeName,
		Versions: []models.ProviderVersion{
			{
				Version: version,
				Platforms: []models.Platform{
					{OS: osName, Arch: arch},
				},
			},
		},
	}

	if err := st.uploadProviderMetadata(ctx, namespace, typeName, metadata); err != nil {
		st.logger.Error("Failed to update provider metadata after upload", "error", err)
		// Don't return error - binary was uploaded successfully
		// Metadata can be fixed later
	}

	return nil
}

// uploadProviderMetadata uploads provider metadata to api.
func (st *Storage) uploadProviderMetadata(ctx context.Context, namespace, typeName string, metadata *models.ProviderMetadata) error {
	key := st.getProviderMetadataKey(namespace, typeName)

	// Try to read existing metadata
	existingMetadata := &models.ProviderMetadata{
		Namespace: namespace,
		Type:      typeName,
		Versions:  []models.ProviderVersion{},
	}

	obj := st.client.Bucket(st.bucket).Object(key)
	reader, err := obj.NewReader(ctx)

	if err == nil {
		// Metadata exists, read and parse it
		defer reader.Close()
		body, readErr := io.ReadAll(reader)
		if readErr != nil {
			return fmt.Errorf("failed to read existing metadata: %w", readErr)
		}

		if unmarshalErr := json.Unmarshal(body, existingMetadata); unmarshalErr != nil {
			st.logger.Warn("Failed to parse existing metadata, will overwrite", "error", unmarshalErr, "key", key)
			// Continue with empty existingMetadata
		}
	} else {
		// Check if error is "not found"
		if err == errObjectNotExist {
			st.logger.Info("No existing metadata found, creating new", "key", key)
		} else {
			st.logger.Warn("Error checking for existing metadata", "error", err)
		}
		// Continue with empty existingMetadata
	}

	// Merge each version from the incoming metadata
	for _, newVersion := range metadata.Versions {
		existingMetadata = models.MergeProviderVersion(existingMetadata, newVersion)
	}

	if err := st.writeJSONToStorage(ctx, key, existingMetadata); err != nil {
		return fmt.Errorf("failed to upload provider metadata: %w", err)
	}

	st.logger.Info("Uploaded merged provider metadata", "key", key, "versions", len(existingMetadata.Versions))
	return nil
}

// uploadModuleArchive uploads a module archive to storage.
// path is the local filesystem path to the module archive file.
func (st *Storage) uploadModuleArchive(ctx context.Context, namespace, name, system, version, path string) error {
	file, err := os.Open(filepath.Clean(path)) // #nosec G304 - path from trusted CLI input
	if err != nil {
		return fmt.Errorf("failed to open module archive: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat module archive: %w", err)
	}

	// Validate it's not empty
	if fileInfo.Size() == 0 {
		return fmt.Errorf("module archive is empty")
	}

	// Calculate MD5 for GCS integrity validation
	md5Hash := md5.New() // #nosec G401 - MD5 required by GCS for integrity
	if _, err := io.Copy(md5Hash, file); err != nil {
		return fmt.Errorf("failed to calculate module archive checksum: %w", err)
	}
	md5sum := md5Hash.Sum(nil)

	// Seek back to beginning for upload
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek module archive: %w", err)
	}

	// Generate key using helper
	key := st.getModuleArchiveKey(namespace, name, system, version)

	// Upload to GCS with MD5 checksum for data integrity
	writer := st.client.Bucket(st.bucket).Object(key).NewWriter(ctx)
	writer.SetContentType("application/x-gzip")
	writer.SetMD5(md5sum)

	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to upload module archive to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	st.logger.Info("Uploaded module archive", "key", key, "size", fileInfo.Size())

	// After successful upload, update metadata
	metadata := &models.ModuleMetadata{
		Namespace: namespace,
		Name:      name,
		System:    system,
		Versions: []models.Version{
			{Version: version},
		},
	}

	if err := st.uploadModuleMetadata(ctx, namespace, name, system, metadata); err != nil {
		st.logger.Error("Failed to update module metadata after upload", "error", err)
		// Don't return error - archive was uploaded successfully
		// Metadata can be fixed later
	}

	return nil
}

// uploadModuleMetadata uploads module metadata to api.
func (st *Storage) uploadModuleMetadata(ctx context.Context, namespace, name, system string, metadata *models.ModuleMetadata) error {
	key := st.getModuleMetadataKey(namespace, name, system)

	// Try to read existing metadata
	existingMetadata := &models.ModuleMetadata{
		Namespace: namespace,
		Name:      name,
		System:    system,
		Versions:  []models.Version{},
	}

	obj := st.client.Bucket(st.bucket).Object(key)
	reader, err := obj.NewReader(ctx)

	if err == nil {
		// Metadata exists, read and parse it
		defer reader.Close()
		body, readErr := io.ReadAll(reader)
		if readErr != nil {
			return fmt.Errorf("failed to read existing module metadata: %w", readErr)
		}

		if unmarshalErr := json.Unmarshal(body, existingMetadata); unmarshalErr != nil {
			st.logger.Warn("Failed to parse existing module metadata, will overwrite", "error", unmarshalErr, "key", key)
			// Continue with empty existingMetadata
		}
	} else {
		// Check if error is "not found"
		if err == errObjectNotExist {
			st.logger.Info("No existing module metadata found, creating new", "key", key)
		} else {
			st.logger.Warn("Error checking for existing module metadata", "error", err)
		}
		// Continue with empty existingMetadata
	}

	// Merge each version from the incoming metadata
	for _, newVersion := range metadata.Versions {
		existingMetadata = models.MergeModuleVersion(existingMetadata, newVersion)
	}

	if err := st.writeJSONToStorage(ctx, key, existingMetadata); err != nil {
		return fmt.Errorf("failed to upload module metadata: %w", err)
	}

	st.logger.Info("Uploaded module metadata", "key", key, "versions", len(existingMetadata.Versions))
	return nil
}

// deleteProvider deletes a provider version from api.
func (st *Storage) deleteProvider(ctx context.Context, namespace, typeName, version string) error {
	// Construct prefix for this version (covers all platforms)
	versionPrefix := fmt.Sprintf("%s/providers/%s/%s/%s/", st.prefix, namespace, typeName, version)

	st.logger.Info("Deleting provider version", "namespace", namespace, "type", typeName, "version", version, "prefix", versionPrefix)

	// List all objects under this version
	var objectsToDelete []string

	bucket := st.client.Bucket(st.bucket)
	it := bucket.Objects(ctx, &gcsstorage.Query{
		Prefix: versionPrefix,
	})

	for {
		attrs, err := it.Next()
		if err == iteratorDone {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list provider objects: %w", err)
		}

		objectsToDelete = append(objectsToDelete, attrs.Name)
	}

	if len(objectsToDelete) == 0 {
		st.logger.Warn("No objects found to delete", "prefix", versionPrefix)
		// Continue anyway to clean up metadata
	}

	// Delete all objects (GCS doesn't have batch delete, so loop)
	for _, key := range objectsToDelete {
		if err := bucket.Object(key).Delete(ctx); err != nil {
			st.logger.Error("Failed to delete object", "key", key, "error", err)
			// Continue with other deletes
		}
	}

	st.logger.Info("Deleted provider objects", "count", len(objectsToDelete))

	// Get existing metadata
	metadataKey := st.getProviderMetadataKey(namespace, typeName)
	obj := bucket.Object(metadataKey)
	reader, err := obj.NewReader(ctx)

	if err != nil {
		// Metadata might not exist - that's OK
		st.logger.Warn("Could not read provider metadata", "error", err)
		return nil
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata models.ProviderMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		st.logger.Error("Failed to parse metadata", "error", err)
		return nil // Don't fail - files are already deleted
	}

	// Remove the version from metadata
	newVersions := make([]models.ProviderVersion, 0, len(metadata.Versions))
	for _, v := range metadata.Versions {
		if v.Version != version {
			newVersions = append(newVersions, v)
		}
	}
	metadata.Versions = newVersions

	if len(metadata.Versions) == 0 {
		// No versions left, delete the metadata file
		if err := obj.Delete(ctx); err != nil {
			st.logger.Error("Failed to delete empty metadata file", "error", err)
		}
		st.logger.Info("Deleted provider metadata (no versions remaining)")
		return nil
	}

	// Update metadata with remaining versions
	if err := st.writeJSONToStorage(ctx, metadataKey, &metadata); err != nil {
		return fmt.Errorf("failed to update provider metadata: %w", err)
	}
	st.logger.Info("Updated provider metadata", "remaining_versions", len(metadata.Versions))
	return nil
}

// deleteModule deletes a module version from api.
func (st *Storage) deleteModule(ctx context.Context, namespace, name, system, version string) error {
	// Delete the archive file
	archiveKey := st.getModuleArchiveKey(namespace, name, system, version)

	st.logger.Info("Deleting module archive", "namespace", namespace, "name", name, "system", system, "version", version)

	bucket := st.client.Bucket(st.bucket)
	if err := bucket.Object(archiveKey).Delete(ctx); err != nil {
		// Log but don't fail - object might not exist
		st.logger.Warn("Module archive not found, may have been deleted already", "key", archiveKey, "error", err)
	}

	// Get existing metadata
	metadataKey := st.getModuleMetadataKey(namespace, name, system)
	metadataObj := bucket.Object(metadataKey)
	reader, err := metadataObj.NewReader(ctx)

	if err != nil {
		st.logger.Warn("Could not read module metadata", "error", err)
		return nil // Archive is deleted, that's the main goal
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read module metadata: %w", err)
	}

	var metadata models.ModuleMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		st.logger.Error("Failed to parse module metadata", "error", err)
		return nil
	}

	// Remove the version
	newVersions := make([]models.Version, 0, len(metadata.Versions))
	for _, v := range metadata.Versions {
		if v.Version != version {
			newVersions = append(newVersions, v)
		}
	}
	metadata.Versions = newVersions

	if len(metadata.Versions) == 0 {
		// Delete metadata file
		if err := metadataObj.Delete(ctx); err != nil {
			st.logger.Error("Failed to delete empty module metadata", "error", err)
		}
		st.logger.Info("Deleted module metadata (no versions remaining)")
		return nil
	}

	// Update metadata with remaining versions
	if err := st.writeJSONToStorage(ctx, metadataKey, &metadata); err != nil {
		return fmt.Errorf("failed to update module metadata: %w", err)
	}
	st.logger.Info("Updated module metadata", "remaining_versions", len(metadata.Versions))
	return nil
}
