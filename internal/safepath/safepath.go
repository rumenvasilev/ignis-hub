// Package safepath provides secure file operations with path traversal prevention.
package safepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OpenFile opens a file after validating it's within the allowed base directory.
// This prevents path traversal attacks (CWE-22) by ensuring the resolved absolute
// path stays within the expected directory boundary.
//
// Parameters:
//   - baseDir: The allowed base directory (e.g., temp dir, upload dir)
//   - path: The path to open (can be absolute or relative)
//
// Returns an error if the path escapes the base directory.
func OpenFile(baseDir, path string) (*os.File, error) {
	if err := ValidateContainment(baseDir, path); err != nil {
		return nil, err
	}
	return os.Open(filepath.Clean(path))
}

// ReadFile reads a file after validating it's within the allowed base directory.
// This is a convenience wrapper around OpenFile for cases where you want the
// entire file contents as a byte slice.
func ReadFile(baseDir, path string) ([]byte, error) {
	if err := ValidateContainment(baseDir, path); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Clean(path))
}

// ValidateContainment checks that a path resolves to within the base directory.
// This is the core security check to prevent path traversal.
func ValidateContainment(baseDir, targetPath string) error {
	// Resolve both paths to absolute
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	// Clean both paths for consistent comparison
	absBase = filepath.Clean(absBase)
	absTarget = filepath.Clean(absTarget)

	// Ensure target is within base directory
	// We add separator to avoid false positives like /tmp/foo matching /tmp/foobar
	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
		return fmt.Errorf("path %s is outside allowed directory %s", targetPath, baseDir)
	}

	return nil
}
