package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"

	"github.com/rumenvasilev/ignis-hub/internal/safepath"
)

const IgnoreFileName = ".ignishubctlignore"

// CreateTgz creates a .tar.gz archive from a directory, respecting .ignishubctlignore patterns.
// Returns the path to the created archive file.
func CreateTgz(srcDir, destFile string) error {
	// Ensure source directory exists
	info, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("source directory error: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcDir)
	}

	// Load ignore patterns
	ignorer, err := loadIgnorePatterns(srcDir)
	if err != nil {
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	// Create output file
	outFile, err := os.Create(filepath.Clean(destFile)) // #nosec G304 - path from trusted CLI input
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk the directory and add files
	srcDir = filepath.Clean(srcDir)
	return filepath.Walk(srcDir, makeWalkFunc(srcDir, ignorer, tarWriter))
}

// makeWalkFunc creates the filepath.WalkFunc for archiving.
func makeWalkFunc(srcDir string, ignorer *ignore.GitIgnore, tarWriter *tar.Writer) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for archive and ignore checking
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Check if path should be ignored
		if shouldIgnore(ignorer, relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		return addToArchive(srcDir, path, relPath, info, tarWriter)
	}
}

// addToArchive adds a single file or directory to the tar archive.
// baseDir is used for path containment validation to prevent path traversal.
func addToArchive(baseDir, path, relPath string, info os.FileInfo, tarWriter *tar.Writer) error {
	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}

	// Use relative path in archive (with forward slashes for compatibility)
	header.Name = filepath.ToSlash(relPath)

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read symlink: %w", err)
		}
		header.Linkname = link
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write file content (only for regular files)
	if info.Mode().IsRegular() {
		file, err := safepath.OpenFile(baseDir, path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file to archive: %w", err)
		}
	}

	return nil
}

// loadIgnorePatterns loads ignore patterns from .ignishubctlignore file.
// Returns nil ignorer if no ignore file exists.
func loadIgnorePatterns(dir string) (*ignore.GitIgnore, error) {
	ignoreFile := filepath.Join(dir, IgnoreFileName)

	// Check if ignore file exists
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		return nil, nil
	}

	// Parse the ignore file
	ignorer, err := ignore.CompileIgnoreFile(ignoreFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", IgnoreFileName, err)
	}

	return ignorer, nil
}

// shouldIgnore checks if a path should be ignored based on patterns.
// Always ignores .git directory and .ignishubctlignore file itself.
func shouldIgnore(ignorer *ignore.GitIgnore, relPath string) bool {
	// Always ignore .git directory
	if relPath == ".git" || strings.HasPrefix(relPath, ".git/") || strings.HasPrefix(relPath, ".git\\") {
		return true
	}

	// Always ignore the ignore file itself
	if relPath == IgnoreFileName {
		return true
	}

	// If no ignorer, don't ignore anything else
	if ignorer == nil {
		return false
	}

	// Check against ignore patterns
	return ignorer.MatchesPath(relPath)
}

// CreateTempTgz creates a temporary .tar.gz archive and returns its path.
// The caller is responsible for removing the temporary file.
func CreateTempTgz(srcDir string) (string, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "module-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Create the archive
	if err := CreateTgz(srcDir, tmpPath); err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return "", err
	}

	return tmpPath, nil
}
