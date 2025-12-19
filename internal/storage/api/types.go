// Package api defines the types, interfaces, and errors for the storage layer.
// This package has no dependencies on storage implementations, avoiding import cycles.
package api

import (
	"context"

	"github.com/rumenvasilev/ignis-hub/internal/models"
)

// ResourceType identifies the type of registry resource.
type ResourceType string

const (
	ResourceTypeModule   ResourceType = "module"
	ResourceTypeProvider ResourceType = "provider"
)

// ProviderFileKind identifies the type of provider file being uploaded.
type ProviderFileKind string

const (
	ProviderFileKindBinary    ProviderFileKind = ""          // Default: provider binary zip
	ProviderFileKindChecksum  ProviderFileKind = "checksum"  // SHA256SUMS file
	ProviderFileKindSignature ProviderFileKind = "signature" // SHA256SUMS.sig file
)

// ResourceIdentifier contains the parameters to identify a resource.
type ResourceIdentifier struct {
	Type      ResourceType
	Namespace string
	Name      string
	// Module-specific
	System string
	// Provider-specific
	OS          string
	Arch        string
	FileKind    ProviderFileKind    // Type of provider file (binary, checksum, signature)
	SigningKeys *models.SigningKeys // Optional signing keys for provider binary uploads
	// Common
	Version string
}

// VersionsResponse contains version metadata for a resource.
type VersionsResponse struct {
	Module   *models.ModuleMetadata
	Provider *models.ProviderMetadata
}

// IsModule returns true if this is a module response.
func (v *VersionsResponse) IsModule() bool {
	return v.Module != nil
}

// IsProvider returns true if this is a provider response.
func (v *VersionsResponse) IsProvider() bool {
	return v.Provider != nil
}

// DownloadInfoResponse contains download information for a resource.
type DownloadInfoResponse struct {
	// For modules: download URL
	URL string
	// For providers: binary metadata
	ProviderBinary *models.ProviderBinaryMetadata
}

// IsModuleURL returns true if this is a module download URL.
func (d *DownloadInfoResponse) IsModuleURL() bool {
	return d.URL != "" && d.ProviderBinary == nil
}

// IsProviderBinary returns true if this is provider binary metadata.
func (d *DownloadInfoResponse) IsProviderBinary() bool {
	return d.ProviderBinary != nil
}

// ResourceInfo represents basic resource information for listing.
type ResourceInfo struct {
	ResourceType ResourceType
	Namespace    string
	Name         string
	System       string // For modules only
	Versions     []string
}

// ProviderInfo represents basic provider information for listing.
type ProviderInfo struct {
	Namespace string
	Type      string // Provider type name (e.g., "aws", "google")
	Versions  []string
}

// ModuleInfo represents basic module information for listing.
type ModuleInfo struct {
	Namespace string
	Name      string
	System    string
	Versions  []string
}

// Reader provides read operations for registry resources.
type Reader interface {
	// GetVersions returns metadata for all versions of a resource.
	GetVersions(ctx context.Context, id ResourceIdentifier) (*VersionsResponse, error)

	// GetDownloadInfo returns download information for a specific version.
	GetDownloadInfo(ctx context.Context, id ResourceIdentifier) (*DownloadInfoResponse, error)

	// List returns all resources of a given type.
	List(ctx context.Context, resourceType ResourceType) ([]ResourceInfo, error)
}

// Writer provides write operations for registry resources.
type Writer interface {
	// Upload uploads a resource artifact (module archive or provider binary).
	Upload(ctx context.Context, id ResourceIdentifier, filepath string) error

	// UpdateMetadata updates the metadata for a resource.
	UpdateMetadata(ctx context.Context, id ResourceIdentifier, metadata interface{}) error

	// Delete removes a specific version of a resource.
	Delete(ctx context.Context, id ResourceIdentifier) error
}

// HealthChecker provides health check capabilities.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// Storage combines all storage operations.
type Storage interface {
	Reader
	Writer
	HealthChecker
}
