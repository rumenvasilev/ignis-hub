package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/matryer/is"
)

func TestCreateTgz_BasicDirectory(t *testing.T) {
	is := is.New(t)

	// Create temp directory with files
	srcDir := t.TempDir()
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "main.tf"), []byte("# main terraform"), 0644))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "variables.tf"), []byte("# variables"), 0644))
	is.NoErr(os.MkdirAll(filepath.Join(srcDir, "modules", "sub"), 0755))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "modules", "sub", "main.tf"), []byte("# submodule"), 0644))

	// Create archive
	destFile := filepath.Join(t.TempDir(), "module.tar.gz")
	err := CreateTgz(srcDir, destFile)
	is.NoErr(err)

	// Verify archive contents
	files := listTgzContents(t, destFile)
	is.True(contains(files, "main.tf"))
	is.True(contains(files, "variables.tf"))
	is.True(contains(files, "modules/sub/main.tf"))
}

func TestCreateTgz_WithIgnoreFile(t *testing.T) {
	is := is.New(t)

	// Create temp directory with files
	srcDir := t.TempDir()
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "main.tf"), []byte("# main"), 0644))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "secret.tfvars"), []byte("password=xxx"), 0644))
	is.NoErr(os.MkdirAll(filepath.Join(srcDir, ".terraform"), 0755))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, ".terraform", "cache"), []byte("cache"), 0644))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# readme"), 0644))

	// Create ignore file
	ignoreContent := `# Ignore secrets
*.tfvars
# Ignore terraform cache
.terraform/
# Ignore docs
README.md
`
	is.NoErr(os.WriteFile(filepath.Join(srcDir, IgnoreFileName), []byte(ignoreContent), 0644))

	// Create archive
	destFile := filepath.Join(t.TempDir(), "module.tar.gz")
	err := CreateTgz(srcDir, destFile)
	is.NoErr(err)

	// Verify archive contents
	files := listTgzContents(t, destFile)
	is.True(contains(files, "main.tf"))           // Should be included
	is.True(!contains(files, "secret.tfvars"))    // Should be ignored
	is.True(!contains(files, ".terraform/cache")) // Should be ignored (whole dir)
	is.True(!contains(files, "README.md"))        // Should be ignored
	is.True(!contains(files, IgnoreFileName))     // Ignore file itself should not be included
}

func TestCreateTgz_AlwaysIgnoresGitDirectory(t *testing.T) {
	is := is.New(t)

	// Create temp directory with .git
	srcDir := t.TempDir()
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "main.tf"), []byte("# main"), 0644))
	is.NoErr(os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755))
	is.NoErr(os.WriteFile(filepath.Join(srcDir, ".git", "config"), []byte("[core]"), 0644))

	// Create archive (no ignore file)
	destFile := filepath.Join(t.TempDir(), "module.tar.gz")
	err := CreateTgz(srcDir, destFile)
	is.NoErr(err)

	// Verify .git is not included
	files := listTgzContents(t, destFile)
	is.True(contains(files, "main.tf"))
	is.True(!contains(files, ".git/config"))
}

func TestCreateTgz_NotADirectory(t *testing.T) {
	is := is.New(t)

	// Create a file instead of directory
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	is.NoErr(os.WriteFile(tmpFile, []byte("content"), 0644))

	destFile := filepath.Join(t.TempDir(), "module.tar.gz")
	err := CreateTgz(tmpFile, destFile)
	is.True(err != nil)
}

func TestCreateTempTgz(t *testing.T) {
	is := is.New(t)

	// Create temp directory with files
	srcDir := t.TempDir()
	is.NoErr(os.WriteFile(filepath.Join(srcDir, "main.tf"), []byte("# main"), 0644))

	// Create temp archive
	archivePath, err := CreateTempTgz(srcDir)
	is.NoErr(err)
	defer os.Remove(archivePath)

	// Verify file exists
	_, err = os.Stat(archivePath)
	is.NoErr(err)

	// Verify it's a valid archive
	files := listTgzContents(t, archivePath)
	is.True(contains(files, "main.tf"))
}

func TestShouldIgnore_Patterns(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected bool
	}{
		{".git directory", ".git", true},
		{".git/config file", ".git/config", true},
		{"ignore file itself", IgnoreFileName, true},
		{"regular file", "main.tf", false},
		{"regular dir", "modules", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			result := shouldIgnore(nil, tt.relPath)
			is.Equal(result, tt.expected)
		})
	}
}

// Helper to list contents of a .tar.gz file
func listTgzContents(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var files []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}
		files = append(files, header.Name)
	}

	return files
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
