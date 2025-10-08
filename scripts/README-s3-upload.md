# S3 Upload Script

This script manages uploading Terraform provider binaries to a real AWS S3 bucket for your private Terraform registry.

## Quick Start

### 1. Set AWS Credentials

```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=us-east-1  # optional, defaults to us-east-1
```

### 2. Download Providers (if not already done)

```bash
./scripts/download-real-providers.sh
```

This will download provider binaries to the `providers/` directory.

### 3. Upload to S3

```bash
./scripts/upload-to-s3.sh upload
```

## Commands

### Status
Check the bucket and see what's uploaded:

```bash
./scripts/upload-to-s3.sh status
```

Output:
```
🔍 S3 Bucket Status:
   Bucket: test-roomba-my-terraform-registry
   Region: us-east-1
   Prefix: registry

✅ Bucket 'test-roomba-my-terraform-registry' exists and is accessible

📊 Object counts by provider:
   hashicorp/aws/                 45 objects
   hashicorp/random/              12 objects

📈 Total objects: 57

💾 Storage usage:
Total Objects: 57
   Total Size: 234.5 MiB
```

### List
List all objects in the bucket:

```bash
./scripts/upload-to-s3.sh list
```

### Upload
Upload provider binaries to S3:

```bash
# Upload from default 'providers' directory
./scripts/upload-to-s3.sh upload

# Upload from custom directory
./scripts/upload-to-s3.sh upload ./my-custom-providers

# Use environment variable
PROVIDERS_DIR=./custom-providers ./scripts/upload-to-s3.sh upload
```

The script will:
- Check that the specified directory exists
- Count the files to upload
- Ask for confirmation
- Upload all files to `s3://test-roomba-my-terraform-registry/registry/providers/`
- Verify the upload

### Download
Download a specific file from S3:

```bash
# Download to current directory with same filename
./scripts/upload-to-s3.sh download providers/hashicorp/aws/5.20.1/metadata.json

# Download to specific file
./scripts/upload-to-s3.sh download providers/hashicorp/aws/5.20.1/metadata.json my-metadata.json
```

### Sync
Sync a local directory to S3 (⚠️ **deletes files in S3 not present locally**):

```bash
./scripts/upload-to-s3.sh sync ./providers
```

**WARNING**: This uses `aws s3 sync --delete`, which will remove files from S3 that don't exist locally!

### Clean
Delete all objects from the bucket (⚠️ **dangerous**):

```bash
./scripts/upload-to-s3.sh clean
```

You must type `DELETE ALL` to confirm. This action cannot be undone!

## Configuration

### Bucket Name
Currently hardcoded to: `test-roomba-my-terraform-registry`

To change, edit the script:
```bash
BUCKET="your-bucket-name"
```

### S3 Prefix
All objects are uploaded under the `registry/` prefix:
```
s3://test-roomba-my-terraform-registry/registry/providers/hashicorp/aws/...
```

To change, edit the script:
```bash
S3_PREFIX="your-prefix"
```

### Region
Defaults to `eu-west-1`. Override with environment variable:
```bash
export AWS_REGION=us-west-2
./scripts/upload-to-s3.sh upload
```

### Providers Directory
Defaults to `providers/`. Override with environment variable or command line argument:

```bash
# Method 1: Environment variable
export PROVIDERS_DIR=./my-providers
./scripts/upload-to-s3.sh upload

# Method 2: Command line argument (takes precedence)
./scripts/upload-to-s3.sh upload ./my-providers

# Method 3: Edit the script
PROVIDERS_DIR="my-custom-directory"
```

## Required AWS Permissions

Your AWS credentials need these permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:GetBucketLocation"
      ],
      "Resource": "arn:aws:s3:::test-roomba-my-terraform-registry"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::test-roomba-my-terraform-registry/*"
    }
  ]
}
```

## Typical Workflow

### Initial Setup

```bash
# 1. Set credentials
export AWS_ACCESS_KEY_ID=AKIAXXXXXXXXXXXXXXXX
export AWS_SECRET_ACCESS_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# 2. Download providers
./scripts/download-real-providers.sh

# 3. Check what will be uploaded
find providers -type f | wc -l

# 4. Upload to S3
./scripts/upload-to-s3.sh upload

# 5. Verify
./scripts/upload-to-s3.sh status
```

### Adding More Providers

```bash
# 1. Edit download-real-providers.sh to add more providers

# 2. Download the new providers
./scripts/download-real-providers.sh

# 3. Upload (only new files will be uploaded)
./scripts/upload-to-s3.sh upload
```

### Checking Status

```bash
# Quick check
./scripts/upload-to-s3.sh status

# Detailed listing
./scripts/upload-to-s3.sh list
```

## Integration with Registry

After uploading, configure your registry application to use this bucket:

### Environment Variables

```bash
export REGISTRY_AWS_REGION=us-east-1
export REGISTRY_AWS_S3_BUCKET=test-roomba-my-terraform-registry
export REGISTRY_AWS_S3_PREFIX=registry
export REGISTRY_AWS_ACCESS_KEY_ID=your-key
export REGISTRY_AWS_SECRET_ACCESS_KEY=your-secret

# Start the registry
go run main.go
```

### Kubernetes ConfigMap/Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: terraform-registry-aws
type: Opaque
stringData:
  aws-access-key-id: "your-key"
  aws-secret-access-key: "your-secret"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: terraform-registry-config
data:
  REGISTRY_AWS_REGION: "us-east-1"
  REGISTRY_AWS_S3_BUCKET: "test-roomba-my-terraform-registry"
  REGISTRY_AWS_S3_PREFIX: "registry"
```

## Troubleshooting

### "AWS credentials not set"

Make sure you've exported the environment variables:
```bash
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
```

### "Bucket does not exist or is not accessible"

1. Check the bucket name is correct
2. Check the region is correct (`export AWS_REGION=...`)
3. Verify you have access:
   ```bash
   aws s3 ls s3://test-roomba-my-terraform-registry
   ```
4. Check your AWS credentials are valid:
   ```bash
   aws sts get-caller-identity
   ```

### "providers directory not found"

Run the download script first:
```bash
./scripts/download-real-providers.sh
```

### Upload is slow

Provider binaries can be large (200+ MB). The upload may take several minutes depending on your internet connection.

To see progress, you can modify the script to remove `--no-progress` flag.

### Need to update the bucket name?

Edit the script:
```bash
vim scripts/upload-to-s3.sh
# Change: BUCKET="test-roomba-my-terraform-registry"
# To:     BUCKET="your-new-bucket-name"
```

Or better yet, make it configurable via environment variable:
```bash
BUCKET="${S3_BUCKET:-test-roomba-my-terraform-registry}"
```

Then:
```bash
export S3_BUCKET=my-custom-bucket
./scripts/upload-to-s3.sh upload
```

## Safety Features

The script includes several safety features:

1. **Credential validation**: Checks env vars exist before running
2. **Bucket verification**: Verifies bucket exists and is accessible
3. **Confirmation prompts**: Asks for confirmation before:
   - Uploading files
   - Syncing (which deletes)
   - Cleaning (which deletes everything)
4. **Verification**: After upload, verifies the object count
5. **No deletion by default**: The `upload` command doesn't delete files, only adds/updates

## Comparison: LocalStack vs Real S3

| Feature | `manage-localstack.sh` | `upload-to-s3.sh` |
|---------|------------------------|-------------------|
| Target | LocalStack (localhost:4566) | Real AWS S3 |
| Credentials | Hardcoded `test/test` | Environment variables |
| Safety | No confirmation needed | Confirmation prompts |
| Cost | Free | AWS S3 charges apply |
| Use case | Local development | Production/staging |

## See Also

- [download-real-providers.sh](./download-real-providers.sh) - Download provider binaries
- [manage-localstack.sh](./manage-localstack.sh) - Manage LocalStack S3 for local dev
- [../infrastructure/README.md](../infrastructure/README.md) - Terraform infrastructure setup
