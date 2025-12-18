package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

// writeJSONToStorage marshals data to JSON and uploads it to S3 with SHA256 checksum validation.
// This is the single point of truth for writing JSON metadata to storage.
func (st *Storage) writeJSONToStorage(ctx context.Context, key string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	hash := sha256.Sum256(jsonData)
	checksumBase64 := base64.StdEncoding.EncodeToString(hash[:])

	_, err = st.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(st.bucket),
		Key:               aws.String(key),
		Body:              bytes.NewReader(jsonData),
		ContentType:       aws.String("application/json"),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(checksumBase64),
	})
	if err != nil {
		return fmt.Errorf("failed to upload JSON to S3: %w", err)
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

// uploadProviderFile uploads the binary to S3 and returns metadata needed for the rest of the process.
// The file is closed when this function returns.
func (st *Storage) uploadProviderFile(ctx context.Context, namespace, typeName, version, osName, arch, filePath string) (filename, key, shasum string, err error) {
	file, err := os.Open(filePath)
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

	// Calculate SHA256 first
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", "", fmt.Errorf("failed to calculate shasum: %w", err)
	}
	hashBytes := hash.Sum(nil)
	shasum = hex.EncodeToString(hashBytes)
	checksumBase64 := base64.StdEncoding.EncodeToString(hashBytes)

	// Seek back to beginning for upload
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", "", fmt.Errorf("failed to seek file: %w", err)
	}

	// Upload to S3 with checksum for data integrity
	if _, err := st.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(st.bucket),
		Key:               aws.String(key),
		Body:              file,
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(checksumBase64),
	}); err != nil {
		return "", "", "", fmt.Errorf("failed to upload to S3: %w", err)
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

	// Generate download URL (standard S3 URL for VPC-internal access)
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

	// Calculate checksum for integrity validation
	metadataHash := sha256.Sum256(platformMetadataJSON)
	metadataChecksumBase64 := base64.StdEncoding.EncodeToString(metadataHash[:])

	// Upload platform metadata with checksum for data integrity
	platformMetadataKey := st.getProviderBinaryMetadataKey(namespace, typeName, version, osName, arch)
	_, err = st.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(st.bucket),
		Key:               aws.String(platformMetadataKey),
		Body:              bytes.NewReader(platformMetadataJSON),
		ContentType:       aws.String("application/json"),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(metadataChecksumBase64),
	})
	if err != nil {
		return fmt.Errorf("failed to upload platform metadata: %w", err)
	}

	st.logger.Info("Uploaded provider platform metadata", "key", platformMetadataKey)
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

	result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(key),
	})

	if err == nil {
		// Metadata exists, read and parse it
		defer result.Body.Close()
		body, readErr := io.ReadAll(result.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read existing metadata: %w", readErr)
		}

		if unmarshalErr := json.Unmarshal(body, existingMetadata); unmarshalErr != nil {
			st.logger.Warn("Failed to parse existing metadata, will overwrite", "error", unmarshalErr, "key", key)
			// Continue with empty existingMetadata
		}
	} else {
		// Check if error is "not found" - that's OK for first upload
		st.logger.Info("No existing metadata found, creating new", "key", key)
		// Continue with empty existingMetadata
	}

	// Merge each version from the incoming metadata
	for _, newVersion := range metadata.Versions {
		existingMetadata = api.MergeProviderVersion(existingMetadata, newVersion)
	}

	if err := st.writeJSONToStorage(ctx, key, existingMetadata); err != nil {
		return fmt.Errorf("failed to upload provider metadata: %w", err)
	}

	st.logger.Info("Uploaded merged provider metadata", "key", key, "versions", len(existingMetadata.Versions))
	return nil
}

// uploadModuleArchive uploads a module archive to api.
func (st *Storage) uploadModuleArchive(ctx context.Context, namespace, name, system, version, filePath string) error {
	// Open the file
	file, err := os.Open(filePath)
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

	// Calculate SHA256 first for integrity validation
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate module archive checksum: %w", err)
	}
	checksumBase64 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// Seek back to beginning for upload
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek module archive: %w", err)
	}

	// Generate key using helper
	key := st.getModuleArchiveKey(namespace, name, system, version)

	// Upload to S3 with checksum for data integrity
	_, err = st.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(st.bucket),
		Key:               aws.String(key),
		Body:              file,
		ContentType:       aws.String("application/x-gzip"),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(checksumBase64),
	})
	if err != nil {
		return fmt.Errorf("failed to upload module archive to S3: %w", err)
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

	result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(key),
	})

	if err == nil {
		// Metadata exists, read and parse it
		defer result.Body.Close()
		body, readErr := io.ReadAll(result.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read existing module metadata: %w", readErr)
		}

		if unmarshalErr := json.Unmarshal(body, existingMetadata); unmarshalErr != nil {
			st.logger.Warn("Failed to parse existing module metadata, will overwrite", "error", unmarshalErr, "key", key)
			// Continue with empty existingMetadata
		}
	} else {
		// Check if error is "not found" - that's OK for first upload
		st.logger.Info("No existing module metadata found, creating new", "key", key)
		// Continue with empty existingMetadata
	}

	// Merge each version from the incoming metadata
	for _, newVersion := range metadata.Versions {
		existingMetadata = api.MergeModuleVersion(existingMetadata, newVersion)
	}

	if err := st.writeJSONToStorage(ctx, key, existingMetadata); err != nil {
		return fmt.Errorf("failed to upload module metadata: %w", err)
	}

	st.logger.Info("Uploaded module metadata", "key", key, "versions", len(existingMetadata.Versions))
	return nil
}

// deleteObjectsByPrefix lists and deletes all objects under a given prefix.
func (st *Storage) deleteObjectsByPrefix(ctx context.Context, prefix string) error {
	var objectsToDelete []string

	paginator := s3.NewListObjectsV2Paginator(st.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(st.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				objectsToDelete = append(objectsToDelete, *obj.Key)
			}
		}
	}

	if len(objectsToDelete) == 0 {
		st.logger.Warn("No objects found to delete", "prefix", prefix)
		return nil
	}

	// Delete objects in batches (S3 DeleteObjects supports up to 1000 per call)
	const batchSize = 1000
	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]
		identifiers := make([]types.ObjectIdentifier, len(batch))
		for j, key := range batch {
			keyCopy := key
			identifiers[j] = types.ObjectIdentifier{Key: &keyCopy}
		}

		if _, err := st.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(st.bucket),
			Delete: &types.Delete{
				Objects: identifiers,
				Quiet:   aws.Bool(false),
			},
		}); err != nil {
			return fmt.Errorf("failed to delete objects: %w", err)
		}

		st.logger.Info("Deleted objects batch", "count", len(batch))
	}

	return nil
}

// deleteProvider deletes a provider version from api.
func (st *Storage) deleteProvider(ctx context.Context, namespace, typeName, version string) error {
	versionPrefix := fmt.Sprintf("%s/providers/%s/%s/%s/", st.prefix, namespace, typeName, version)
	st.logger.Info("Deleting provider version", "namespace", namespace, "type", typeName, "version", version)

	if err := st.deleteObjectsByPrefix(ctx, versionPrefix); err != nil {
		return fmt.Errorf("failed to delete provider objects: %w", err)
	}

	// Get existing metadata
	metadataKey := st.getProviderMetadataKey(namespace, typeName)
	result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(metadataKey),
	})

	if err != nil {
		// Metadata might not exist - that's OK
		st.logger.Warn("Could not read provider metadata", "error", err)
		return nil
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
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
		if _, err = st.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(metadataKey),
		}); err != nil {
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

	_, err := st.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(archiveKey),
	})
	if err != nil {
		// Log but don't fail - object might not exist
		st.logger.Warn("Module archive not found, may have been deleted already", "key", archiveKey, "error", err)
	}

	// Get existing metadata
	metadataKey := st.getModuleMetadataKey(namespace, name, system)
	result, err := st.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.bucket),
		Key:    aws.String(metadataKey),
	})

	if err != nil {
		st.logger.Warn("Could not read module metadata", "error", err)
		return nil // Archive is deleted, that's the main goal
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
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
		if _, err = st.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(st.bucket),
			Key:    aws.String(metadataKey),
		}); err != nil {
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
