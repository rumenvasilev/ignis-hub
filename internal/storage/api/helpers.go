package api

import "github.com/rumenvasilev/ignis-hub/internal/models"

// MergeProviderVersion merges a new version into existing provider metadata.
// If the version already exists, it replaces it. Otherwise, it appends it.
func MergeProviderVersion(existing *models.ProviderMetadata, newVersion models.ProviderVersion) *models.ProviderMetadata {
	// Find if version already exists
	existingIndex := -1
	for i, v := range existing.Versions {
		if v.Version == newVersion.Version {
			existingIndex = i
			break
		}
	}

	if existingIndex >= 0 {
		// Replace existing version
		existing.Versions[existingIndex] = newVersion
	} else {
		// Append new version
		existing.Versions = append(existing.Versions, newVersion)
	}

	return existing
}

// MergeModuleVersion merges a new version into existing module metadata.
// If the version already exists, it replaces it. Otherwise, it appends it.
func MergeModuleVersion(existing *models.ModuleMetadata, newVersion models.Version) *models.ModuleMetadata {
	// Find if version already exists
	existingIndex := -1
	for i, v := range existing.Versions {
		if v.Version == newVersion.Version {
			existingIndex = i
			break
		}
	}

	if existingIndex >= 0 {
		// Replace existing version
		existing.Versions[existingIndex] = newVersion
	} else {
		// Append new version
		existing.Versions = append(existing.Versions, newVersion)
	}

	return existing
}
