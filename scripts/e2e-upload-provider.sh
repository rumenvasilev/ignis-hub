#!/bin/bash

# E2E Provider Upload Test Script
# Creates and uploads a test provider to verify the upload workflow
set -e

AWS_ENDPOINT="${AWS_ENDPOINT:-http://localhost:4566}"
AWS_REGION="${AWS_REGION:-us-east-1}"
S3_BUCKET="${S3_BUCKET:-terraform-registry}"
S3_PREFIX="${S3_PREFIX:-registry}"

# Test provider details
NAMESPACE="test-namespace"
PROVIDER_TYPE="test-provider"
VERSION="1.0.0"
OS="linux"
ARCH="amd64"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}ℹ️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Create temporary directory for test files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

log_info "Creating test provider files in $TEMP_DIR"

# Create a dummy provider binary
BINARY_FILE="$TEMP_DIR/terraform-provider-${PROVIDER_TYPE}_v${VERSION}"
echo "#!/bin/bash" > "$BINARY_FILE"
echo "echo 'Test provider binary'" >> "$BINARY_FILE"
chmod +x "$BINARY_FILE"

# Calculate SHA256
SHASUM=$(sha256sum "$BINARY_FILE" | awk '{print $1}')
log_info "Provider binary SHA256: $SHASUM"

# Create provider metadata JSON
METADATA_FILE="$TEMP_DIR/metadata.json"
cat > "$METADATA_FILE" << EOF
{
  "versions": [
    {
      "version": "${VERSION}",
      "protocols": ["5.0"],
      "platforms": [
        {
          "os": "${OS}",
          "arch": "${ARCH}"
        }
      ]
    }
  ]
}
EOF

log_info "Provider metadata created"

# Create binary metadata JSON
BINARY_METADATA_FILE="$TEMP_DIR/binary_metadata.json"
DOWNLOAD_URL="http://localhost:8080/download/${NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/${OS}/${ARCH}/terraform-provider-${PROVIDER_TYPE}_${VERSION}_${OS}_${ARCH}.zip"

cat > "$BINARY_METADATA_FILE" << EOF
{
  "arch": "${ARCH}",
  "download_url": "${DOWNLOAD_URL}",
  "filename": "terraform-provider-${PROVIDER_TYPE}_${VERSION}_${OS}_${ARCH}.zip",
  "os": "${OS}",
  "protocols": ["5.0"],
  "shasum": "${SHASUM}",
  "shasums_url": "${DOWNLOAD_URL}.shasums",
  "shasums_signature_url": "${DOWNLOAD_URL}.shasums.sig",
  "signing_keys": {
    "gpg_public_keys": []
  }
}
EOF

log_info "Binary metadata created"

# Set AWS credentials for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=$AWS_REGION

# Determine which AWS CLI to use (awslocal if available, otherwise aws with endpoint)
if command -v awslocal &> /dev/null; then
    log_info "Using awslocal CLI"
    AWS_CMD="awslocal"
else
    log_info "Using aws CLI with endpoint: $AWS_ENDPOINT"
    AWS_CMD="aws --endpoint-url=$AWS_ENDPOINT"
fi

# Upload to S3 (LocalStack)
log_info "Uploading provider metadata to S3..."

# Upload provider-level metadata
PROVIDER_METADATA_KEY="${S3_PREFIX}/providers/${NAMESPACE}/${PROVIDER_TYPE}/metadata.json"
$AWS_CMD s3 cp "$METADATA_FILE" "s3://${S3_BUCKET}/${PROVIDER_METADATA_KEY}" || {
    log_error "Failed to upload provider metadata"
    exit 1
}
log_info "✅ Uploaded: $PROVIDER_METADATA_KEY"

# Upload binary metadata
BINARY_METADATA_KEY="${S3_PREFIX}/providers/${NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/${OS}/${ARCH}/metadata.json"
$AWS_CMD s3 cp "$BINARY_METADATA_FILE" "s3://${S3_BUCKET}/${BINARY_METADATA_KEY}" || {
    log_error "Failed to upload binary metadata"
    exit 1
}
log_info "✅ Uploaded: $BINARY_METADATA_KEY"

# Upload binary (optional, for completeness)
BINARY_KEY="${S3_PREFIX}/providers/${NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/${OS}/${ARCH}/terraform-provider-${PROVIDER_TYPE}_${VERSION}_${OS}_${ARCH}.zip"
# Create a zip of the binary
(cd "$TEMP_DIR" && zip -q "provider.zip" "$(basename $BINARY_FILE)")
$AWS_CMD s3 cp "$TEMP_DIR/provider.zip" "s3://${S3_BUCKET}/${BINARY_KEY}" || {
    log_warning "Failed to upload binary (non-critical)"
}
log_info "✅ Uploaded: $BINARY_KEY"

# Verify uploads
log_info "Verifying uploads in S3..."
$AWS_CMD s3 ls "s3://${S3_BUCKET}/${S3_PREFIX}/providers/${NAMESPACE}/${PROVIDER_TYPE}/" --recursive

log_info "✅ Provider upload test completed successfully!"
echo ""
echo "Provider details:"
echo "  Namespace: $NAMESPACE"
echo "  Type: $PROVIDER_TYPE"
echo "  Version: $VERSION"
echo "  Platform: ${OS}/${ARCH}"
echo ""
echo "Test the provider via API:"
echo "  curl http://localhost:8080/v1/providers/${NAMESPACE}/${PROVIDER_TYPE}/versions"
echo "  curl http://localhost:8080/v1/providers/${NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/download/${OS}/${ARCH}"

