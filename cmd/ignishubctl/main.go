package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/rumenvasilev/ignis-hub/internal/archive"
	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/safepath"
	"github.com/rumenvasilev/ignis-hub/internal/storage"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

// version is set via ldflags at build time
var version = "dev"

const (
	officialRegistryURL = "https://registry.terraform.io"
	defaultTempDir      = "/tmp/ignishubctl"
)

// Platform definitions for provider downloads
var defaultPlatforms = []struct {
	OS   string
	Arch string
}{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
}

func main() {
	app := cliApp()

	if err := app.Run(os.Args); err != nil {
		slog.Error("Failed to run CLI", "error", err)
		os.Exit(1)
	}
}

func cliApp() *cli.App {
	return &cli.App{
		Name:    "ignishubctl",
		Usage:   "Command line tool for managing Ignis Hub Terraform registry",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "provider",
				Aliases: []string{"p"},
				Value:   "aws",
				Usage:   "Storage provider (aws or gcp)",
				EnvVars: []string{"REGISTRY_PROVIDER"},
			},
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "",
				Usage:   "Path to config file",
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "list-providers",
				Aliases: []string{"lp"},
				Usage:   "List all available providers",
				Action:  listProviders,
			},
			{
				Name:    "list-modules",
				Aliases: []string{"lm"},
				Usage:   "List all available modules",
				Action:  listModules,
			},
			{
				Name:    "list-providers-versions",
				Aliases: []string{"lpv"},
				Usage:   "List versions for a specific provider",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Provider namespace",
					},
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Required: true,
						Usage:    "Provider type",
					},
				},
				Action: listProviderVersions,
			},
			{
				Name:    "list-modules-versions",
				Aliases: []string{"lmv"},
				Usage:   "List versions for a specific module",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Module namespace",
					},
					&cli.StringFlag{
						Name:     "name",
						Required: true,
						Usage:    "Module name",
					},
					&cli.StringFlag{
						Name:     "system",
						Aliases:  []string{"s"},
						Required: true,
						Usage:    "Module system",
					},
				},
				Action: listModuleVersions,
			},
			{
				Name:  "add-provider",
				Usage: "Add a provider to the registry",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Provider namespace",
					},
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Required: true,
						Usage:    "Provider type",
					},
					&cli.StringFlag{
						Name:     "version",
						Aliases:  []string{"v"},
						Required: true,
						Usage:    "Provider version",
					},
					&cli.StringFlag{
						Name:     "os",
						Required: true,
						Usage:    "Operating system (e.g., linux, darwin, windows)",
					},
					&cli.StringFlag{
						Name:     "arch",
						Required: true,
						Usage:    "Architecture (e.g., amd64, arm64)",
					},
					&cli.StringFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Required: true,
						Usage:    "Path to provider binary file",
					},
				},
				Action: addProvider,
			},
			{
				Name:  "add-module",
				Usage: "Add a module to the registry",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Module namespace",
					},
					&cli.StringFlag{
						Name:     "name",
						Required: true,
						Usage:    "Module name",
					},
					&cli.StringFlag{
						Name:     "system",
						Aliases:  []string{"s"},
						Required: true,
						Usage:    "Module system",
					},
					&cli.StringFlag{
						Name:     "version",
						Aliases:  []string{"v"},
						Required: true,
						Usage:    "Module version",
					},
					&cli.StringFlag{
						Name:    "file",
						Aliases: []string{"f"},
						Usage:   "Path to module archive file (.tar.gz)",
					},
					&cli.StringFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Usage:   "Path to module directory (will be archived, respects .ignishubctlignore)",
					},
				},
				Action: addModule,
			},
			{
				Name:  "remove-provider",
				Usage: "Remove a provider from the registry",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Provider namespace",
					},
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Required: true,
						Usage:    "Provider type",
					},
					&cli.StringFlag{
						Name:     "version",
						Aliases:  []string{"v"},
						Required: true,
						Usage:    "Provider version",
					},
				},
				Action: removeProvider,
			},
			{
				Name:  "remove-module",
				Usage: "Remove a module from the registry",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Module namespace",
					},
					&cli.StringFlag{
						Name:     "name",
						Required: true,
						Usage:    "Module name",
					},
					&cli.StringFlag{
						Name:     "system",
						Aliases:  []string{"s"},
						Required: true,
						Usage:    "Module system",
					},
					&cli.StringFlag{
						Name:     "version",
						Aliases:  []string{"v"},
						Required: true,
						Usage:    "Module version",
					},
				},
				Action: removeModule,
			},
			{
				Name:  "clone-provider",
				Usage: "Clone a provider from official Terraform registry to your registry",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "namespace",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Provider namespace (e.g., hashicorp)",
					},
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Required: true,
						Usage:    "Provider type (e.g., aws)",
					},
					&cli.StringFlag{
						Name:     "version",
						Aliases:  []string{"v"},
						Required: true,
						Usage:    "Provider version (e.g., 5.20.1)",
					},
					&cli.StringSliceFlag{
						Name:    "platforms",
						Aliases: []string{"pl"},
						Usage:   "Specific platforms to clone (format: os/arch, e.g., linux/amd64). If not specified, clones all default platforms",
					},
					&cli.StringFlag{
						Name:  "temp-dir",
						Value: defaultTempDir,
						Usage: "Temporary directory for downloads",
					},
					&cli.BoolFlag{
						Name:  "keep-temp",
						Value: false,
						Usage: "Keep temporary downloaded files after upload",
					},
				},
				Action: cloneProvider,
			},
		},
	}
}

// getStorage initializes and returns a storage instance based on the CLI context
func getStorage(c *cli.Context) (api.Storage, error) {
	ctx := context.Background()

	// Load configuration
	provider := c.String("provider")
	cfg, err := config.Load(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only show warnings and errors for CLI
	}))

	// Create storage based on provider
	store, err := storage.New(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return store, nil
}

// Command implementations

func listProviders(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	resources, err := store.List(context.Background(), api.ResourceTypeProvider)
	if err != nil {
		return fmt.Errorf("failed to list providers: %w", err)
	}

	if len(resources) == 0 {
		fmt.Println("No providers found")
		return nil
	}

	// Print results in a formatted table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tVERSIONS")
	fmt.Fprintln(w, "--------\t--------")

	var versionsMsg string
	for _, resource := range resources {
		versionsMsg = ""
		if len(resource.Versions) > 0 {
			versionsMsg = fmt.Sprintf("%d version(s)", len(resource.Versions))
		} else {
			versionsMsg = "No versions"
		}
		// Format: namespace/type (matches Terraform provider source format)
		fmt.Fprintf(w, "%s/%s\t%s\n", resource.Namespace, resource.Name, versionsMsg)
	}

	_ = w.Flush() // #nosec G104 - stdout flush error is non-critical
	return nil
}

func listModules(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	resources, err := store.List(context.Background(), api.ResourceTypeModule)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	if len(resources) == 0 {
		fmt.Println("No modules found")
		return nil
	}

	// Print results in a formatted table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "MODULE\tVERSIONS")
	fmt.Fprintln(w, "------\t--------")

	var versionsMsg string
	for _, resource := range resources {
		versionsMsg = ""
		if len(resource.Versions) > 0 {
			versionsMsg = fmt.Sprintf("%d version(s)", len(resource.Versions))
		} else {
			versionsMsg = "No versions"
		}
		// Format: namespace/name/system (matches Terraform module source format)
		fmt.Fprintf(w, "%s/%s/%s\t%s\n", resource.Namespace, resource.Name, resource.System, versionsMsg)
	}

	_ = w.Flush() // #nosec G104 - stdout flush error is non-critical
	return nil
}

func listProviderVersions(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	typeName := c.String("type")

	versionsResp, err := store.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: namespace,
		Name:      typeName,
	})
	if err != nil {
		return fmt.Errorf("failed to get provider versions: %w", err)
	}

	if versionsResp.Provider == nil || len(versionsResp.Provider.Versions) == 0 {
		fmt.Printf("No versions found for provider %s/%s\n", namespace, typeName)
		return nil
	}

	metadata := versionsResp.Provider
	fmt.Printf("Provider: %s/%s\n\n", namespace, typeName)

	// Print results in a formatted table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VERSION\tPROTOCOLS\tPLATFORMS")
	fmt.Fprintln(w, "-------\t---------\t---------")

	var protocolsMsg, platformsMsg string
	for _, version := range metadata.Versions {
		protocolsMsg = ""
		if len(version.Protocols) > 0 {
			protocolsMsg = fmt.Sprintf("%v", version.Protocols)
		}

		platformsMsg = ""
		if len(version.Platforms) > 0 {
			platformsMsg = fmt.Sprintf("%d platform(s)", len(version.Platforms))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", version.Version, protocolsMsg, platformsMsg)
	}

	_ = w.Flush() // #nosec G104 - stdout flush error is non-critical
	return nil
}

func listModuleVersions(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	name := c.String("name")
	system := c.String("system")

	versionsResp, err := store.GetVersions(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: namespace,
		Name:      name,
		System:    system,
	})
	if err != nil {
		return fmt.Errorf("failed to get module versions: %w", err)
	}

	if versionsResp.Module == nil || len(versionsResp.Module.Versions) == 0 {
		fmt.Printf("No versions found for module %s/%s/%s\n", namespace, name, system)
		return nil
	}

	metadata := versionsResp.Module
	fmt.Printf("Module: %s/%s/%s\n\n", namespace, name, system)

	// Print results in a formatted table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VERSION")
	fmt.Fprintln(w, "-------")

	for _, version := range metadata.Versions {
		fmt.Fprintf(w, "%s\n", version.Version)
	}

	_ = w.Flush() // #nosec G104 - stdout flush error is non-critical
	return nil
}

func addProvider(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	typeName := c.String("type")
	version := c.String("version")
	osName := c.String("os")
	arch := c.String("arch")
	filePath := c.String("file")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// validate the file is a valid provider binary
	if !isValidProviderBinary(filePath) {
		return fmt.Errorf("file is not a valid provider binary")
	}

	fmt.Printf("Adding provider %s/%s version %s for %s/%s...\n", namespace, typeName, version, osName, arch)

	err = store.Upload(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: namespace,
		Name:      typeName,
		Version:   version,
		OS:        osName,
		Arch:      arch,
	}, filePath)
	if err != nil {
		return fmt.Errorf("failed to upload provider: %w", err)
	}

	fmt.Println("Provider added successfully")
	return nil
}

func isValidProviderBinary(filePath string) bool {
	// TODO: Implement provider binary validation
	// TODO: Implement name,os,arch validation

	// Validate the filename format
	// terraform-provider-fastssm_5.100.0_linux_arm64.zip
	ver := regexp.MustCompile(`terraform-provider-([a-zA-Z0-9_-]+)_(\d+\.\d+\.\d+)_(linux|darwin|windows)_(amd64|arm64).zip`)
	matches := ver.FindStringSubmatch(filePath)
	return len(matches) != 0
}

func addModule(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	name := c.String("name")
	system := c.String("system")
	version := c.String("version")
	filePath := c.String("file")
	dirPath := c.String("dir")

	// Validate that exactly one of --file or --dir is provided
	if filePath == "" && dirPath == "" {
		return fmt.Errorf("either --file or --dir must be specified")
	}
	if filePath != "" && dirPath != "" {
		return fmt.Errorf("only one of --file or --dir can be specified, not both")
	}

	// Default to using the provided file path (no cleanup needed)
	archivePath := filePath
	cleanup := func() {}

	// If directory provided, create archive instead
	if dirPath != "" {
		// Verify directory exists
		info, err := os.Stat(dirPath)
		if err != nil {
			return fmt.Errorf("directory error: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", dirPath)
		}

		fmt.Printf("Archiving module from %s...\n", dirPath)

		// Check for .ignishubctlignore
		ignoreFile := filepath.Join(dirPath, archive.IgnoreFileName)
		if _, err := os.Stat(ignoreFile); err == nil {
			fmt.Printf("Using ignore patterns from %s\n", archive.IgnoreFileName)
		}

		// Create temporary archive
		archivePath, err = archive.CreateTempTgz(dirPath)
		if err != nil {
			return fmt.Errorf("failed to create archive: %w", err)
		}
		cleanup = func() { _ = os.Remove(archivePath) } // #nosec G104 - cleanup error is non-critical

		// Get archive size for display
		if info, err := os.Stat(archivePath); err == nil {
			fmt.Printf("Created archive: %s (%.2f KB)\n", filepath.Base(archivePath), float64(info.Size())/1024)
		}
	}
	defer cleanup()

	// Safety check: verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	fmt.Printf("Adding module %s/%s/%s version %s...\n", namespace, name, system, version)

	err = store.Upload(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: namespace,
		Name:      name,
		System:    system,
		Version:   version,
	}, archivePath)
	if err != nil {
		return fmt.Errorf("failed to upload module: %w", err)
	}

	fmt.Println("Module added successfully")
	return nil
}

func removeProvider(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	typeName := c.String("type")
	version := c.String("version")

	fmt.Printf("Removing provider %s/%s version %s...\n", namespace, typeName, version)

	err = store.Delete(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: namespace,
		Name:      typeName,
		Version:   version,
	})
	if err != nil {
		return fmt.Errorf("failed to remove provider: %w", err)
	}

	fmt.Println("Provider removed successfully")
	return nil
}

func removeModule(c *cli.Context) error {
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	namespace := c.String("namespace")
	name := c.String("name")
	system := c.String("system")
	version := c.String("version")

	fmt.Printf("Removing module %s/%s/%s version %s...\n", namespace, name, system, version)

	err = store.Delete(context.Background(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: namespace,
		Name:      name,
		System:    system,
		Version:   version,
	})
	if err != nil {
		return fmt.Errorf("failed to remove module: %w", err)
	}

	fmt.Println("Module removed successfully")
	return nil
}

func cloneProvider(c *cli.Context) error {
	namespace := c.String("namespace")
	typeName := c.String("type")
	version := c.String("version")
	tempDir := c.String("temp-dir")
	keepTemp := c.Bool("keep-temp")
	platformsFlag := c.StringSlice("platforms")

	fmt.Printf("🚀 Cloning provider %s/%s version %s from official registry...\n\n", namespace, typeName, version)

	// Parse platforms
	var platforms []struct {
		OS   string
		Arch string
	}

	if len(platformsFlag) > 0 {
		for _, p := range platformsFlag {
			parts := strings.Split(p, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid platform format '%s', expected os/arch (e.g., linux/amd64)", p)
			}
			platforms = append(platforms, struct {
				OS   string
				Arch string
			}{OS: parts[0], Arch: parts[1]})
		}
	} else {
		platforms = defaultPlatforms
	}

	// Create temp directory
	providerTempDir := filepath.Join(tempDir, namespace, typeName, version)
	if err := os.MkdirAll(providerTempDir, 0750); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	if !keepTemp {
		defer func() {
			fmt.Printf("\n🧹 Cleaning up temporary files...\n")
			_ = os.RemoveAll(filepath.Join(tempDir, namespace)) // #nosec G104 - cleanup error is non-critical
		}()
	}

	// Download provider for each platform
	var downloadedPlatforms []models.Platform
	var checksumFiles []string
	httpClient := &http.Client{Timeout: 5 * time.Minute}

	for i, platform := range platforms {
		fmt.Printf("[%d/%d] 📦 Downloading for %s/%s...\n", i+1, len(platforms), platform.OS, platform.Arch)

		downloaded, err := downloadProviderPlatform(httpClient, namespace, typeName, version, platform.OS, platform.Arch, providerTempDir)
		if err != nil {
			fmt.Printf("  ⚠️  Failed to download %s/%s: %v\n", platform.OS, platform.Arch, err)
			continue
		}

		downloadedPlatforms = append(downloadedPlatforms, models.Platform{
			OS:   platform.OS,
			Arch: platform.Arch,
		})

		if downloaded.ChecksumFile != "" && !contains(checksumFiles, downloaded.ChecksumFile) {
			checksumFiles = append(checksumFiles, downloaded.ChecksumFile)
		}
		if downloaded.ChecksumSigFile != "" && !contains(checksumFiles, downloaded.ChecksumSigFile) {
			checksumFiles = append(checksumFiles, downloaded.ChecksumSigFile)
		}

		fmt.Printf("  ✅ Downloaded successfully\n")
	}

	if len(downloadedPlatforms) == 0 {
		return fmt.Errorf("failed to download any platforms for provider")
	}

	fmt.Printf("\n✅ Downloaded %d platform(s)\n\n", len(downloadedPlatforms))

	// Get storage and upload
	fmt.Println("☁️  Uploading to registry storage...")
	store, err := getStorage(c)
	if err != nil {
		return err
	}

	// Upload all downloaded files
	if err := uploadProvider(store, namespace, typeName, version, providerTempDir, downloadedPlatforms); err != nil {
		return fmt.Errorf("failed to upload provider: %w", err)
	}

	fmt.Printf("\n🎉 Successfully cloned provider %s/%s version %s\n", namespace, typeName, version)
	fmt.Printf("   Platforms: %d\n", len(downloadedPlatforms))

	if keepTemp {
		fmt.Printf("   Temp files kept at: %s\n", providerTempDir)
	}

	return nil
}

type downloadedProviderInfo struct {
	BinaryFile      string
	MetadataFile    string
	ChecksumFile    string
	ChecksumSigFile string
}

func downloadProviderPlatform(client *http.Client, namespace, name, version, osName, arch, tempDir string) (*downloadedProviderInfo, error) {
	// Get download info from official registry
	downloadURL := fmt.Sprintf("%s/v1/providers/%s/%s/%s/download/%s/%s",
		officialRegistryURL, namespace, name, version, osName, arch)

	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch download info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var downloadInfo struct {
		Arch                string          `json:"arch"`
		DownloadURL         string          `json:"download_url"`
		Filename            string          `json:"filename"`
		OS                  string          `json:"os"`
		Protocols           []string        `json:"protocols"`
		Shasum              string          `json:"shasum"`
		ShasumsURL          string          `json:"shasums_url"`
		ShasumsSignatureURL string          `json:"shasums_signature_url"`
		SigningKeys         json.RawMessage `json:"signing_keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&downloadInfo); err != nil {
		return nil, fmt.Errorf("failed to decode download info: %w", err)
	}

	// Create platform directory
	platformDir := filepath.Join(tempDir, osName, arch)
	if err := os.MkdirAll(platformDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create platform directory: %w", err)
	}

	result := &downloadedProviderInfo{}

	// Step 1: Download SHA256SUMS file first (required for verification)
	checksumFilename := fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS", name, version)
	checksumPath := filepath.Join(tempDir, checksumFilename)

	// Only download if we don't already have it (shared across platforms)
	if _, err := os.Stat(checksumPath); os.IsNotExist(err) {
		if downloadInfo.ShasumsURL == "" {
			return nil, fmt.Errorf("no SHA256SUMS URL available - cannot verify integrity")
		}
		if err := downloadFile(client, downloadInfo.ShasumsURL, checksumPath); err != nil {
			return nil, fmt.Errorf("failed to download SHA256SUMS: %w", err)
		}
	}
	result.ChecksumFile = checksumPath

	// Step 2: Download signature file (optional, for future GPG verification)
	if downloadInfo.ShasumsSignatureURL != "" {
		checksumSigFilename := fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS.sig", name, version)
		checksumSigPath := filepath.Join(tempDir, checksumSigFilename)
		if _, err := os.Stat(checksumSigPath); os.IsNotExist(err) {
			if err := downloadFile(client, downloadInfo.ShasumsSignatureURL, checksumSigPath); err == nil {
				result.ChecksumSigFile = checksumSigPath
			}
		} else {
			result.ChecksumSigFile = checksumSigPath
		}
	}

	// Step 3: Parse SHA256SUMS to get expected checksum for this binary
	checksums, err := parseChecksumFile(tempDir, checksumPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SHA256SUMS: %w", err)
	}
	expectedChecksum, ok := checksums[downloadInfo.Filename]
	if !ok {
		return nil, fmt.Errorf("checksum for %s not found in SHA256SUMS file", downloadInfo.Filename)
	}

	// Step 4: Download the binary - validate filename doesn't escape temp directory
	binaryPath := filepath.Join(platformDir, downloadInfo.Filename)
	if err := safepath.ValidateContainment(tempDir, binaryPath); err != nil {
		return nil, fmt.Errorf("invalid filename from registry (possible path traversal): %w", err)
	}
	if err := downloadFile(client, downloadInfo.DownloadURL, binaryPath); err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}

	// Step 5: Verify the binary against checksum from SHA256SUMS file
	if err := verifyFileChecksum(tempDir, binaryPath, expectedChecksum); err != nil {
		// Remove the corrupted/tampered file
		_ = os.Remove(binaryPath)
		return nil, fmt.Errorf("checksum verification failed for %s: %w", downloadInfo.Filename, err)
	}
	fmt.Printf("  ✓ Checksum verified for %s\n", downloadInfo.Filename)

	result.BinaryFile = binaryPath

	// Create metadata.json for this platform
	metadata := map[string]interface{}{
		"os":           downloadInfo.OS,
		"arch":         downloadInfo.Arch,
		"filename":     downloadInfo.Filename,
		"shasum":       downloadInfo.Shasum,
		"protocols":    downloadInfo.Protocols,
		"signing_keys": downloadInfo.SigningKeys,
	}

	metadataPath := filepath.Join(platformDir, "metadata.json")
	metadataFile, err := os.Create(metadataPath) // #nosec G304 - path constructed from known directory
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metadataFile.Close()

	encoder := json.NewEncoder(metadataFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}
	result.MetadataFile = metadataPath

	return result, nil
}

// parseChecksumFile reads a SHA256SUMS file and returns a map of filename -> checksum.
// The file format is: "<sha256_hash>  <filename>" (two spaces between hash and filename).
// baseDir is used for path containment validation.
func parseChecksumFile(baseDir, path string) (map[string]string, error) {
	data, err := safepath.ReadFile(baseDir, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksum file: %w", err)
	}

	checksums := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<hash>  <filename>" (two spaces)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			// Try single space as fallback
			parts = strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
		}
		hash := strings.TrimSpace(parts[0])
		filename := strings.TrimSpace(parts[1])
		// Handle *filename format (binary mode indicator)
		filename = strings.TrimPrefix(filename, "*")
		checksums[filename] = hash
	}

	if len(checksums) == 0 {
		return nil, fmt.Errorf("no valid checksums found in file")
	}

	return checksums, nil
}

// verifyFileChecksum computes the SHA256 hash of a file and compares it
// against the expected checksum. This ensures the downloaded file hasn't
// been tampered with or corrupted during transfer.
// baseDir is used for path containment validation.
// path is the local filesystem path to the downloaded file.
func verifyFileChecksum(baseDir, path, expectedChecksum string) error {
	file, err := safepath.OpenFile(baseDir, path)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// allowedDownloadHosts contains the permitted hosts for provider downloads.
// These are the known hosts used by HashiCorp and common provider registries.
var allowedDownloadHosts = map[string]bool{
	"releases.hashicorp.com":        true,
	"github.com":                    true,
	"objects.githubusercontent.com": true,
	"registry.terraform.io":         true,
	"registry.opentofu.org":         true,
}

// validateDownloadURL ensures the URL is safe to fetch (prevents SSRF attacks).
func validateDownloadURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Require HTTPS
	if parsed.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS scheme, got: %s", parsed.Scheme)
	}

	// Check against allowlist
	if !allowedDownloadHosts[parsed.Host] {
		return fmt.Errorf("host %q not in allowed download hosts", parsed.Host)
	}

	// Reject private IP ranges (defense in depth)
	if ip := net.ParseIP(parsed.Hostname()); ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("URL points to private/local IP address")
		}
	}

	return nil
}

func downloadFile(client *http.Client, downloadURL, destPath string) error {
	// Validate URL to prevent SSRF attacks
	if err := validateDownloadURL(downloadURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath) // #nosec G304 - callers validate path containment or construct filename
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func uploadProvider(store api.Storage, namespace, typeName, version, tempDir string, platforms []models.Platform) error {
	ctx := context.Background()

	// Upload each platform's files
	for _, platform := range platforms {
		platformDir := filepath.Join(tempDir, platform.OS, platform.Arch)

		// Find the binary file
		entries, err := os.ReadDir(platformDir)
		if err != nil {
			return fmt.Errorf("failed to read platform directory: %w", err)
		}

		var binaryFile string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
				binaryFile = filepath.Join(platformDir, entry.Name())
				break
			}
		}

		if binaryFile == "" {
			return fmt.Errorf("no binary file found for %s/%s", platform.OS, platform.Arch)
		}

		// Upload the binary
		fmt.Printf("  📤 Uploading %s/%s binary...\n", platform.OS, platform.Arch)
		if err := store.Upload(ctx, api.ResourceIdentifier{
			Type:      api.ResourceTypeProvider,
			Namespace: namespace,
			Name:      typeName,
			Version:   version,
			OS:        platform.OS,
			Arch:      platform.Arch,
		}, binaryFile); err != nil {
			return fmt.Errorf("failed to upload binary for %s/%s: %w", platform.OS, platform.Arch, err)
		}
	}

	// Upload checksum files (if they exist)
	checksumFile := filepath.Join(tempDir, fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS", typeName, version))
	if _, err := os.Stat(checksumFile); err == nil {
		fmt.Printf("  📤 Uploading checksum file...\n")
		// Note: We'd need a method to upload checksum files, for now just log
	}

	checksumSigFile := filepath.Join(tempDir, fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS.sig", typeName, version))
	if _, err := os.Stat(checksumSigFile); err == nil {
		fmt.Printf("  📤 Uploading checksum signature...\n")
		// Note: We'd need a method to upload checksum signature files
	}

	// Create and upload provider metadata
	providerMetadata := &models.ProviderMetadata{
		Namespace: namespace,
		Type:      typeName,
		Versions: []models.ProviderVersion{
			{
				Version:   version,
				Protocols: []string{"5.0"},
				Platforms: platforms,
			},
		},
	}

	fmt.Printf("  📤 Uploading provider metadata...\n")
	if err := store.UpdateMetadata(ctx, api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: namespace,
		Name:      typeName,
	}, providerMetadata); err != nil {
		return fmt.Errorf("failed to upload provider metadata: %w", err)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
