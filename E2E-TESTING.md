# End-to-End Testing Guide

This document describes the E2E testing setup for the Ignis Hub Terraform Registry.

## Overview

The E2E tests verify the complete workflow of the registry:
1. **Infrastructure**: LocalStack (S3) + Registry service
2. **Data Upload**: Provider/module metadata and binaries
3. **API Testing**: All registry endpoints
4. **Integration**: Terraform client compatibility

## Running E2E Tests Locally

### Prerequisites

- Docker and Docker Compose
- `jq` for JSON parsing
- `curl` for API testing
- AWS CLI (for manual S3 operations)

### Quick Start

```bash
# Start the full stack
docker-compose up -d

# Wait for services to be ready
./scripts/e2e-test.sh

# Upload a test provider
./scripts/e2e-upload-provider.sh

# Run API tests
./scripts/test-api.sh
```

### Step-by-Step

1. **Start LocalStack and Registry**:
```bash
docker-compose up -d
```

This starts:
- LocalStack (S3 emulation) on port 4566
- Terraform Registry on port 8080
- Initializes S3 bucket with sample data

2. **Verify Services**:
```bash
# Check health
curl http://localhost:8080/health

# Check LocalStack
aws --endpoint-url=http://localhost:4566 s3 ls s3://terraform-registry/registry/ --recursive
```

3. **Run E2E Tests**:
```bash
./scripts/e2e-test.sh
```

This tests:
- ✅ Health check endpoint
- ✅ Well-known discovery endpoint
- ✅ Module version listing
- ✅ Module download URLs
- ✅ Provider version listing
- ✅ Provider binary metadata
- ✅ Root endpoint
- ✅ CORS handling
- ✅ 404 responses
- ✅ API versioning

4. **Test Provider Upload**:
```bash
./scripts/e2e-upload-provider.sh
```

This creates and uploads a test provider to verify:
- S3 upload workflow
- Metadata structure
- Binary handling
- API retrieval

5. **Verify Uploaded Provider**:
```bash
# List versions
curl http://localhost:8080/v1/providers/test-namespace/test-provider/versions | jq

# Get binary metadata
curl http://localhost:8080/v1/providers/test-namespace/test-provider/1.0.0/download/linux/amd64 | jq
```

## GitHub Actions E2E Workflow

The E2E tests run automatically in CI/CD via `.github/workflows/e2e-test.yml`.

### Workflow Steps

1. **Setup Go**: Install Go 1.25.3
2. **Start LocalStack**: Uses [LocalStack GitHub Action](https://github.com/LocalStack/setup-localstack) for S3 emulation
3. **Initialize S3**: Create bucket and upload test data using `awslocal`
4. **Build & Start Registry**: Compile and launch the registry service in background
5. **Run E2E Tests**: Execute comprehensive API test suite
6. **Upload Test**: Verify provider upload workflow
7. **Verify**: Confirm uploaded data is accessible via API
8. **Terraform Integration**: Test with actual Terraform client
9. **Cleanup**: Stop registry process

### Key Features

- **No Docker Compose**: Uses native LocalStack GitHub Action (faster, cleaner)
- **Background Process**: Registry runs as background process with PID tracking
- **awslocal CLI**: Automatically installed by LocalStack action
- **Log Capture**: Registry logs saved to `registry.log` for debugging
- **Artifact Upload**: Logs uploaded on failure for troubleshooting

### Triggering the Workflow

The E2E workflow runs on:
- Push to any branch (except main)
- Pull requests to main
- Manual trigger via `workflow_dispatch`

```bash
# Trigger manually via GitHub CLI
gh workflow run e2e-test.yml
```

## Test Scripts

### `scripts/e2e-test.sh`

Comprehensive API testing script that verifies all registry endpoints.

**Features**:
- Colored output (pass/fail indicators)
- Automatic retry/wait for service readiness
- Detailed error reporting
- Exit codes for CI/CD integration

**Usage**:
```bash
# Default (localhost:8080)
./scripts/e2e-test.sh

# Custom base URL
BASE_URL=http://registry.example.com ./scripts/e2e-test.sh
```

### `scripts/e2e-upload-provider.sh`

Creates and uploads a test provider to verify the upload workflow.

**What it does**:
1. Creates a dummy provider binary
2. Generates metadata JSON files
3. Calculates SHA256 checksums
4. Uploads to S3 (LocalStack)
5. Verifies uploads

**Usage**:
```bash
# Default settings
./scripts/e2e-upload-provider.sh

# Custom settings
AWS_ENDPOINT=http://localhost:4566 \
S3_BUCKET=my-bucket \
./scripts/e2e-upload-provider.sh
```

### `scripts/e2e-verify-provider.sh`

Verifies that an uploaded provider is accessible via the registry API.

**What it checks**:
1. Provider versions endpoint responds
2. Uploaded version exists in version list
3. Binary metadata endpoint responds
4. Required fields (download_url, shasum) are present

**Usage**:
```bash
# Default (test-namespace/test-provider)
./scripts/e2e-verify-provider.sh

# Custom provider
NAMESPACE=hashicorp \
PROVIDER_TYPE=aws \
VERSION=5.20.1 \
./scripts/e2e-verify-provider.sh
```

### `scripts/e2e-terraform-integration.sh`

Tests Terraform client integration with the registry.

**What it does**:
1. Creates a Terraform configuration
2. Configures Terraform CLI to use the registry
3. Runs `terraform init`
4. Verifies provider resolution works

**Usage**:
```bash
# Default
./scripts/e2e-terraform-integration.sh

# Custom registry URL
BASE_URL=http://registry.example.com ./scripts/e2e-terraform-integration.sh
```

### `scripts/test-api.sh`

Quick API smoke tests for manual verification.

**Usage**:
```bash
./scripts/test-api.sh
```

## Docker Compose Configuration

The `docker-compose.yml` defines the E2E environment:

```yaml
services:
  terraform-registry:  # Main registry service
  localstack:          # S3 emulation
  init-localstack:     # One-time initialization
  aws-cli:             # Optional CLI for debugging
```

### Key Features

- **Health Checks**: Ensures services are ready before tests
- **Persistence**: LocalStack data persists in `./tmp/localstack`
- **Sample Data**: Automatically uploads from `./examples`
- **Networking**: Isolated bridge network
- **Debugging**: AWS CLI container for manual operations

## Debugging E2E Tests

### View Logs

```bash
# All services
docker-compose logs

# Specific service
docker-compose logs terraform-registry
docker-compose logs localstack

# Follow logs
docker-compose logs -f terraform-registry
```

### Inspect S3 Data

```bash
# Use the AWS CLI container
docker-compose run --rm aws-cli

# Inside the container
aws --endpoint-url=http://localstack:4566 s3 ls s3://terraform-registry/registry/ --recursive
```

### Manual API Testing

```bash
# Health check
curl -v http://localhost:8080/health

# Well-known
curl http://localhost:8080/.well-known/terraform.json | jq

# Provider versions
curl http://localhost:8080/v1/providers/hashicorp/aws/versions | jq

# Module versions
curl http://localhost:8080/v1/modules/terraform-aws-modules/vpc/aws/versions | jq
```

### Common Issues

**Registry not starting**:
```bash
# Check if LocalStack is healthy
docker-compose ps localstack

# Restart services
docker-compose restart terraform-registry
```

**S3 data not loading**:
```bash
# Re-run initialization
docker-compose up init-localstack

# Verify bucket contents
docker-compose run --rm aws-cli aws --endpoint-url=http://localstack:4566 s3 ls s3://terraform-registry/registry/ --recursive
```

**Port conflicts**:
```bash
# Check if ports are in use
lsof -i :8080
lsof -i :4566

# Stop conflicting services or change ports in docker-compose.yml
```

## CI/CD Integration

### Required Secrets

- `LOCALSTACK_API_KEY` - Optional, only needed for LocalStack Pro features (not required for basic S3)

The E2E tests use LocalStack's free tier and don't require real AWS credentials.

### Workflow Outputs

- **Test Results**: Pass/fail status for each test
- **Logs**: Uploaded as artifacts on failure
- **Coverage**: Combined with unit test coverage

### Extending Tests

To add new E2E tests:

1. **Add test function** to `scripts/e2e-test.sh`:
```bash
test_my_new_feature() {
    log_info "Test: My New Feature"
    
    response=$(curl -sf "$BASE_URL/my-endpoint" || echo "FAILED")
    
    if [[ "$response" == "FAILED" ]]; then
        test_failed "My feature test failed"
        return 1
    fi
    
    test_passed "My feature works"
}
```

2. **Call it** in the `main()` function:
```bash
main() {
    # ... existing tests ...
    test_my_new_feature
}
```

3. **Update workflow** if needed (`.github/workflows/e2e-test.yml`)

## Performance Considerations

- **LocalStack GitHub Action**: Starts in ~5-10 seconds (faster than docker-compose)
- **Registry Build**: ~10-20 seconds (Go compilation)
- **Registry Startup**: ~2-5 seconds
- **Full E2E Suite**: Runs in ~30-60 seconds
- **CI/CD Total**: ~1-2 minutes including setup (improved with GitHub Action)

## Best Practices

1. **Idempotent Tests**: Tests should not depend on order
2. **Cleanup**: Always clean up test data
3. **Timeouts**: Use reasonable timeouts for service readiness
4. **Error Messages**: Provide clear failure messages
5. **Isolation**: Each test should be independent

## Related Documentation

- [README.md](README.md) - Main project documentation
- [README-local-development.md](README-local-development.md) - Local development guide
- [QUICK-START-S3.md](QUICK-START-S3.md) - S3 setup guide
- [docker-compose.yml](docker-compose.yml) - Service definitions

