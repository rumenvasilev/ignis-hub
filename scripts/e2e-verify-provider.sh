#!/bin/bash

# E2E Provider Verification Script
# Verifies that uploaded provider is accessible via the registry API
set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
NAMESPACE="${NAMESPACE:-test-namespace}"
PROVIDER_TYPE="${PROVIDER_TYPE:-test-provider}"
VERSION="${VERSION:-1.0.0}"
OS="${OS:-linux}"
ARCH="${ARCH:-amd64}"

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

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_info "Verifying uploaded provider: ${NAMESPACE}/${PROVIDER_TYPE}"

# Check if provider versions are available
log_info "Checking provider versions..."
response=$(curl -sf "${BASE_URL}/v1/providers/${NAMESPACE}/${PROVIDER_TYPE}/versions" || echo "FAILED")

if [[ "$response" == "FAILED" ]]; then
    log_error "Failed to fetch provider versions"
    exit 1
fi

echo "$response" | jq .

# Verify the version we uploaded exists
if echo "$response" | jq -e ".versions[] | select(.version == \"${VERSION}\")" > /dev/null; then
    log_success "Provider version ${VERSION} found"
else
    log_error "Provider version ${VERSION} not found"
    echo "Available versions:"
    echo "$response" | jq '.versions[].version'
    exit 1
fi

# Check if provider binary metadata is downloadable
log_info "Checking provider binary metadata..."
binary_response=$(curl -sf "${BASE_URL}/v1/providers/${NAMESPACE}/${PROVIDER_TYPE}/${VERSION}/download/${OS}/${ARCH}" || echo "FAILED")

if [[ "$binary_response" == "FAILED" ]]; then
    log_error "Failed to fetch provider binary metadata"
    exit 1
fi

echo "$binary_response" | jq .

# Verify required fields exist
if echo "$binary_response" | jq -e '.download_url' > /dev/null; then
    log_success "Provider binary metadata found with download_url"
else
    log_error "Provider binary metadata missing download_url"
    exit 1
fi

if echo "$binary_response" | jq -e '.shasum' > /dev/null; then
    log_success "Provider binary metadata includes shasum"
else
    log_error "Provider binary metadata missing shasum"
    exit 1
fi

log_success "Provider verification completed successfully!"
echo ""
echo "Provider details:"
echo "  Namespace: ${NAMESPACE}"
echo "  Type: ${PROVIDER_TYPE}"
echo "  Version: ${VERSION}"
echo "  Platform: ${OS}/${ARCH}"
echo "  Download URL: $(echo "$binary_response" | jq -r '.download_url')"

