# ignishubctl

Command line tool for managing the Ignis Hub Terraform registry.

## Features

✅ **Full CRUD Operations**
- Upload, list, and delete Terraform providers and modules
- Automatic metadata generation and management
- Version merging (new versions don't overwrite existing ones)

✅ **Clone from Official Registry**
- Download providers from registry.terraform.io
- Automatic multi-platform support
- Upload directly to your private registry

✅ **Multi-Cloud Storage**
- AWS S3 (fully supported)
- Google Cloud Storage (fully supported)

✅ **CI/CD Ready**
- Environment variable configuration
- Exit codes for automation
- Minimal dependencies

✅ **Safety Features**
- Metadata merging preserves all versions
- Per-platform SHA256 checksums
- Presigned download URLs

## Quick Start

```bash
# Build the CLI
go build -o ignishubctl cmd/ignishubctl/main.go

# Configure for AWS S3
export REGISTRY_PROVIDER=aws
export REGISTRY_AWS_S3_BUCKET=my-terraform-registry
export REGISTRY_AWS_REGION=us-east-1

# Clone a provider from official registry
./ignishubctl clone-provider -n hashicorp -t aws -v 5.20.1

# List what you have
./ignishubctl list-providers
./ignishubctl list-providers-versions -n hashicorp -t aws
```

## Installation

Build the CLI from source:

```bash
go build -o ignishubctl cmd/ignishubctl/main.go
```

Or install directly:

```bash
go install github.com/rumenvasilev/ignis-hub/cmd/ignishubctl@latest
```

## Configuration

The CLI uses the same configuration as the main Ignis Hub server. You can configure it using:

1. **Environment variables** (recommended for CLI usage):
   ```bash
   export REGISTRY_PROVIDER=aws  # or gcp
   export REGISTRY_AWS_S3_BUCKET=your-bucket-name
   export REGISTRY_AWS_REGION=us-east-1
   export REGISTRY_AWS_ACCESS_KEY_ID=your-access-key
   export REGISTRY_AWS_SECRET_ACCESS_KEY=your-secret-key
   ```

2. **Config file** (config.yaml in current directory):
   ```yaml
   aws:
     s3_bucket: your-bucket-name
     region: us-east-1
     s3_prefix: registry
   ```

3. **Command line flags**:
   ```bash
   ignishubctl --provider aws --config /path/to/config.yaml list-providers
   ```

## Usage

### List Commands

#### List all providers
```bash
ignishubctl list-providers
# or use the alias
ignishubctl lp
```

#### List all modules
```bash
ignishubctl list-modules
# or use the alias
ignishubctl lm
```

#### List versions for a specific provider
```bash
ignishubctl list-providers-versions --namespace hashicorp --type aws
# or use the alias
ignishubctl lpv -n hashicorp -t aws
```

#### List versions for a specific module
```bash
ignishubctl list-modules-versions --namespace example --name vpc --system aws
# or use the alias
ignishubctl lmv -n example --name vpc -s aws
```

### Add Commands

#### Add a provider
```bash
ignishubctl add-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.0 \
  --os linux \
  --arch amd64 \
  --file /path/to/terraform-provider-aws_5.20.0_linux_amd64.zip
```

#### Add a module
```bash
ignishubctl add-module \
  --namespace example \
  --name vpc \
  --system aws \
  --version 1.0.0 \
  --file /path/to/module-archive.tar.gz
```

### Remove Commands

#### Remove a provider
```bash
ignishubctl remove-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.0
```

#### Remove a module
```bash
ignishubctl remove-module \
  --namespace example \
  --name vpc \
  --system aws \
  --version 1.0.0
```

## Global Options

- `--provider, -p`: Storage provider (aws or gcp), default: aws
- `--config, -c`: Path to config file
- `--help, -h`: Show help
- `--version, -v`: Print version

## Examples

### Managing Providers with AWS S3

```bash
# Set up environment
export REGISTRY_PROVIDER=aws
export REGISTRY_AWS_S3_BUCKET=my-terraform-registry
export REGISTRY_AWS_REGION=us-east-1
export REGISTRY_AWS_ACCESS_KEY_ID=your-key
export REGISTRY_AWS_SECRET_ACCESS_KEY=your-secret

# List all providers
ignishubctl list-providers

# List versions for the AWS provider
ignishubctl lpv -n hashicorp -t aws

# Add a new provider binary
ignishubctl add-provider \
  -n hashicorp -t aws -v 5.30.0 \
  --os darwin --arch arm64 \
  -f ./providers/hashicorp/aws/5.30.0/darwin/arm64/terraform-provider-aws_5.30.0_darwin_arm64.zip
```

### Managing Modules with GCP

```bash
# Set up environment
export REGISTRY_PROVIDER=gcp
export REGISTRY_GCS_BUCKET=my-terraform-registry
export REGISTRY_GCP_CREDENTIALS_FILE=/path/to/credentials.json

# List all modules
ignishubctl list-modules

# Add a new module
ignishubctl add-module \
  -n myorg --name network -s gcp -v 2.0.0 \
  -f ./modules/myorg/network/gcp/2.0.0/archive.tar.gz
```

### Clone Provider from Official Registry

The `clone-provider` command downloads a provider from the official Terraform registry and uploads it to your private registry:

```bash
# Clone a specific provider version with all default platforms
ignishubctl clone-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.1

# Clone for specific platforms only
ignishubctl clone-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.1 \
  --platforms linux/amd64,darwin/arm64

# Keep temporary files for inspection
ignishubctl clone-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.1 \
  --keep-temp

# Specify custom temp directory
ignishubctl clone-provider \
  --namespace hashicorp \
  --type aws \
  --version 5.20.1 \
  --temp-dir /tmp/my-downloads
```

**Default platforms:**
- darwin/amd64
- darwin/arm64
- linux/amd64
- linux/arm64
- windows/amd64

## Storage Backend Support

The CLI fully supports both storage backends:

- ✅ **AWS S3**: Complete support for all operations (list, add, remove, clone)
- ✅ **Google Cloud Storage (GCS)**: Complete support for all operations (list, add, remove, clone)

### What Happens When You Upload

**Providers:**
- Binary file is uploaded to `{prefix}/providers/{namespace}/{type}/{version}/{os}/{arch}/{filename}`
- Per-platform metadata.json is generated with SHA256 hash and download URL
- Root metadata.json is updated by **merging** the new version (existing versions are preserved)
- If you re-upload the same version, it replaces that version in the metadata

**Modules:**
- Archive is uploaded to `{prefix}/modules/{namespace}/{name}/{system}/{version}/archive.tar.gz`
- Root metadata.json is updated by **merging** the new version (existing versions are preserved)

### What Happens When You Delete

**Providers:**
- All platform binaries and metadata for the version are deleted
- The version is removed from root metadata.json
- If no versions remain, the root metadata.json is deleted

**Modules:**
- The archive file is deleted
- The version is removed from root metadata.json
- If no versions remain, the root metadata.json is deleted

## CI/CD Integration

`ignishubctl` is designed for CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Upload Terraform Module
  env:
    REGISTRY_PROVIDER: aws
    REGISTRY_AWS_S3_BUCKET: ${{ secrets.REGISTRY_BUCKET }}
    REGISTRY_AWS_REGION: us-east-1
    REGISTRY_AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    REGISTRY_AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  run: |
    # Package module
    tar -czf module.tar.gz -C ./terraform .

    # Upload to registry
    ignishubctl add-module \
      -n myorg \
      --name vpc \
      -s aws \
      -v ${{ github.ref_name }} \
      -f module.tar.gz
```

## Integration with Scripts

The CLI replaces the need for manual scripts in most cases:

- ✅ Use `ignishubctl clone-provider` instead of `scripts/download-real-providers.sh`
- ✅ Use `ignishubctl add-provider` and `add-module` instead of `scripts/upload-to-s3.sh`
- ✅ Use `ignishubctl` for all day-to-day management operations

## Architecture

The CLI uses the same storage layer as the Ignis Hub server, ensuring consistency between the web API and CLI operations. It supports:

- **Storage abstraction**: Works with both AWS S3 and GCP Cloud Storage
- **Configuration management**: Unified configuration with the main server
- **Error handling**: Consistent error messages and logging
- **Extensibility**: Easy to add new commands and features

## Development

To add new commands:

1. Add the command definition to the `Commands` slice in `main.go`
2. Implement the handler function (e.g., `listProviders`, `addProvider`)
3. Use the `getStorage()` helper to access the storage backend
4. Follow the existing patterns for error handling and output formatting

## Troubleshooting

### Authentication Errors

If you get authentication errors:
- Verify your AWS credentials are correctly set
- Check that your GCP credentials file exists and is valid
- Ensure your IAM roles/service accounts have the necessary permissions

### Storage Bucket Not Found

- Verify the bucket name is correct
- Check that the bucket exists in the specified region
- Ensure you have permission to access the bucket

### Module/Provider Not Found

- Use `list-providers` or `list-modules` to verify the item exists
- Check the namespace, name, and system parameters
- Ensure the metadata files are present in storage

### Version Already Exists

When uploading a provider/module version that already exists:
- The new upload **replaces** the existing version
- All existing versions are preserved (metadata merging)
- This is safe and allows you to update/fix a version

## Implementation Notes

### Metadata Merging

The storage layer intelligently merges metadata:
- **First upload**: Creates new metadata.json with version 1.0.0
- **Second upload**: Reads existing metadata, adds version 2.0.0, preserves 1.0.0
- **Re-upload same version**: Replaces version 1.0.0 in metadata, preserves other versions
- **Delete version**: Removes version from metadata, preserves other versions
- **Delete last version**: Deletes the entire metadata.json file

This ensures you never lose historical version data accidentally.

### Per-Platform Metadata

When uploading a provider binary, the CLI automatically:
1. Uploads the binary to `{prefix}/providers/{namespace}/{type}/{version}/{os}/{arch}/{filename}`
2. Calculates SHA256 hash of the file
3. Generates a presigned download URL (valid for 1 year)
4. Creates and uploads platform-specific metadata.json with hash and URL
5. Updates the root metadata.json by merging the new version

This matches the Terraform Registry Protocol requirements.

### Storage Structure

```
{prefix}/
├── providers/
│   └── {namespace}/
│       └── {type}/
│           ├── metadata.json                    # Root metadata (all versions)
│           └── {version}/
│               └── {os}/
│                   └── {arch}/
│                       ├── {provider}.zip       # Binary
│                       └── metadata.json        # Platform metadata
└── modules/
    └── {namespace}/
        └── {name}/
            └── {system}/
                ├── metadata.json                # Root metadata (all versions)
                └── {version}/
                    └── archive.tar.gz           # Module archive
```

## Version

Current version: **0.1.0**

All core features are fully implemented and production-ready.

## License

See the main repository LICENSE file.

