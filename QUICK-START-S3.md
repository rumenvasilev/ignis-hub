# Quick Start: Upload Providers to S3

This guide walks you through uploading Terraform providers to your real S3 bucket.

## Prerequisites

- AWS account with S3 access
- AWS credentials (access key + secret key)
- S3 bucket: `test-roomba-my-terraform-registry`

## Step-by-Step

### 1. Set AWS Credentials

```bash
export AWS_ACCESS_KEY_ID=your-actual-key
export AWS_SECRET_ACCESS_KEY=your-actual-secret
export AWS_REGION=us-east-1
```

### 2. Download Providers (First Time Only)

```bash
./scripts/download-real-providers.sh
```

This downloads provider binaries to the `providers/` directory.

Expected output:
```
📥 Downloading hashicorp/aws version 5.20.1...
✅ Downloaded successfully
...
```

### 3. Check Status (Optional)

Verify the bucket exists and check what's currently uploaded:

```bash
./scripts/upload-to-s3.sh status
```

### 4. Upload to S3

```bash
# Upload from default 'providers' directory
./scripts/upload-to-s3.sh upload

# Or upload from a custom directory
./scripts/upload-to-s3.sh upload ./my-custom-providers
```

The script will:
1. Count the files to upload
2. Ask for confirmation
3. Upload all files
4. Verify the upload

Example output:
```
📤 Uploading provider data to S3...
📊 Found 57 files to upload
Upload 57 files to s3://test-roomba-my-terraform-registry/registry/providers/? (y/N) y
⏳ Uploading...
✅ Upload completed successfully

📊 Verifying upload...
✅ Total objects in S3: 57
```

### 5. Verify

List uploaded files:

```bash
./scripts/upload-to-s3.sh list
```

Or check status:

```bash
./scripts/upload-to-s3.sh status
```

## Next Steps

### Option A: Run Registry Locally

```bash
export REGISTRY_AWS_REGION=us-east-1
export REGISTRY_AWS_S3_BUCKET=test-roomba-my-terraform-registry
export REGISTRY_AWS_S3_PREFIX=registry
export REGISTRY_AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID
export REGISTRY_AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY
export REGISTRY_AUTH_ENABLED=false

go run main.go
```

### Option B: Deploy to Kubernetes

See [infrastructure/README.md](infrastructure/README.md) for setting up the infrastructure and deploying to Kubernetes.

## Common Commands

```bash
# Check bucket status
./scripts/upload-to-s3.sh status

# List all files
./scripts/upload-to-s3.sh list

# Upload providers (default directory)
./scripts/upload-to-s3.sh upload

# Upload from custom directory
./scripts/upload-to-s3.sh upload ./my-providers

# Download a specific file
./scripts/upload-to-s3.sh download providers/hashicorp/aws/5.20.1/metadata.json

# Get help
./scripts/upload-to-s3.sh help
```

## Troubleshooting

### "AWS credentials not set"

Make sure you exported the environment variables in your current shell:

```bash
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
```

### "Bucket does not exist"

Verify the bucket name and region:

```bash
aws s3 ls s3://test-roomba-my-terraform-registry --region us-east-1
```

### "providers directory not found"

Run the download script first:

```bash
./scripts/download-real-providers.sh
```

## Cost Considerations

Uploading providers to S3 incurs AWS charges:

- **Storage**: ~$0.023/GB/month (Standard S3)
- **Requests**: $0.005 per 1,000 PUT requests
- **Data Transfer**: First 1 GB/month free, then $0.09/GB

Example for 100 providers (~5 GB):
- Storage: ~$0.12/month
- Initial upload: ~$0.025 (one-time)
- **Total first month**: ~$0.15

Very affordable for a private registry!

## Related Documentation

- [scripts/README-s3-upload.md](scripts/README-s3-upload.md) - Full script documentation
- [infrastructure/README.md](infrastructure/README.md) - Infrastructure setup with Terraform
- [infrastructure/USAGE-EXAMPLES.md](infrastructure/USAGE-EXAMPLES.md) - Terraform usage examples
