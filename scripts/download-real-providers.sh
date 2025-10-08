#!/bin/bash

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Default Configuration
EXAMPLES_DIR="examples"
PROVIDERS_REGISTRY="https://registry.terraform.io"
USE_S3=false
S3_BUCKET="test-roomba-my-terraform-registry"
S3_REGION="eu-west-1"

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --use-s3)
            USE_S3=true
            shift
            ;;
        --s3-bucket)
            S3_BUCKET="$2"
            shift 2
            ;;
        --s3-region)
            S3_REGION="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --use-s3              Use S3 URLs instead of LocalStack URLs"
            echo "  --s3-bucket BUCKET    S3 bucket name (default: test-roomba-my-terraform-registry)"
            echo "  --s3-region REGION    S3 region (default: us-east-1)"
            echo "  --help, -h            Show this help message"
            echo ""
            echo "Examples:"
            echo "  # Download with LocalStack URLs (default)"
            echo "  $0"
            echo ""
            echo "  # Download with S3 URLs"
            echo "  $0 --use-s3"
            echo ""
            echo "  # Download with custom S3 bucket"
            echo "  $0 --use-s3 --s3-bucket my-bucket --s3-region eu-west-1"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo "📦 Downloading Real Terraform Provider Binaries"
echo "=============================================="

if [ "$USE_S3" = true ]; then
    echo -e "${BLUE}🪣 Using S3 URLs: s3://${S3_BUCKET} (${S3_REGION})${NC}"
else
    echo -e "${BLUE}🐳 Using LocalStack URLs${NC}"
fi
echo ""

# Provider definitions - using a different approach for better compatibility
PROVIDERS_LIST=(
    "hashicorp:aws:5.100.0"
    "hashicorp:random:3.5.1"
    "rumenvasilev:fastssm:0.1.6"
)

# Platform definitions
PLATFORMS=(
    "darwin_amd64"
    "darwin_arm64" 
    "linux_amd64"
    "linux_arm64"
    "windows_amd64"
)

# Function to download provider for a specific platform
download_provider() {
    local namespace=$1
    local name=$2
    local version=$3
    local platform_os=$4
    local platform_arch=$5
    
    echo -e "${YELLOW}📦 Downloading ${namespace}/${name} v${version} for ${platform_os}/${platform_arch}...${NC}"
    
    # Create directory structure
    local provider_dir="${EXAMPLES_DIR}/providers/${namespace}/${name}/${version}/${platform_os}/${platform_arch}"
    mkdir -p "$provider_dir"
    
    # Get download URL from official registry
    local download_info_url="${PROVIDERS_REGISTRY}/v1/providers/${namespace}/${name}/${version}/download/${platform_os}/${platform_arch}"
    
    echo "🔍 Fetching download info from: $download_info_url"
    local download_response=$(curl -s "$download_info_url")
    
    if [ $? -ne 0 ] || [ -z "$download_response" ]; then
        echo -e "${RED}❌ Failed to get download info for ${namespace}/${name} ${platform_os}/${platform_arch}${NC}"
        return 1
    fi
    
    # Check if the response contains an error
    if echo "$download_response" | grep -q '"errors"'; then
        echo -e "${RED}❌ Provider not available for ${platform_os}/${platform_arch}${NC}"
        echo "   Response: $download_response"
        return 1
    fi
    
    # Extract download URL and filename
    local download_url=$(echo "$download_response" | jq -r '.download_url // empty')
    local filename=$(echo "$download_response" | jq -r '.filename // empty')
    local shasum=$(echo "$download_response" | jq -r '.shasum // empty')
    local shasums_url=$(echo "$download_response" | jq -r '.shasums_url // empty')
    local shasums_signature_url=$(echo "$download_response" | jq -r '.shasums_signature_url // empty')
    local signing_keys=$(echo "$download_response" | jq -c '.signing_keys // {}')
    
    if [ -z "$download_url" ] || [ "$download_url" = "null" ]; then
        echo -e "${RED}❌ No download URL found${NC}"
        return 1
    fi
    
    echo "📁 Download URL: $download_url"
    echo "📄 Filename: $filename"
    
    # Download the binary
    echo "⬇️  Downloading binary..."
    if curl -L -s "$download_url" -o "${provider_dir}/${filename}"; then
        echo -e "${GREEN}✅ Downloaded ${filename}${NC}"
    else
        echo -e "${RED}❌ Failed to download binary${NC}"
        return 1
    fi
    
    # Download checksum files if available
    local version_dir="${EXAMPLES_DIR}/providers/${namespace}/${name}/${version}"
    mkdir -p "$version_dir"
    
    if [ -n "$shasums_url" ] && [ "$shasums_url" != "null" ]; then
        local shasums_filename="terraform-provider-${name}_${version}_SHA256SUMS"
        echo "⬇️  Downloading checksums..."
        if curl -L -s "$shasums_url" -o "${version_dir}/${shasums_filename}"; then
            echo -e "${GREEN}✅ Downloaded ${shasums_filename}${NC}"
        else
            echo -e "${YELLOW}⚠️  Failed to download checksums (continuing anyway)${NC}"
        fi
    fi
    
    if [ -n "$shasums_signature_url" ] && [ "$shasums_signature_url" != "null" ]; then
        local shasums_sig_filename="terraform-provider-${name}_${version}_SHA256SUMS.sig"
        echo "⬇️  Downloading checksums signature..."
        if curl -L -s "$shasums_signature_url" -o "${version_dir}/${shasums_sig_filename}"; then
            echo -e "${GREEN}✅ Downloaded ${shasums_sig_filename}${NC}"
        else
            echo -e "${YELLOW}⚠️  Failed to download checksums signature (continuing anyway)${NC}"
        fi
    fi
    
    # Create metadata.json file
    echo "📝 Creating metadata.json..."
    
    # Determine base URL based on USE_S3 flag
    local base_url
    if [ "$USE_S3" = true ]; then
        base_url="https://${S3_BUCKET}.s3.${S3_REGION}.amazonaws.com/registry/providers"
    else
        base_url="http://localhost:4566/terraform-registry/registry/providers"
    fi
    
    # Create metadata using jq to properly escape the signing keys
    jq -n \
        --arg os "${platform_os}" \
        --arg arch "${platform_arch}" \
        --arg filename "${filename}" \
        --arg download_url "${base_url}/${namespace}/${name}/${version}/${platform_os}/${platform_arch}/${filename}" \
        --arg shasum "${shasum}" \
        --arg shasums_url "${base_url}/${namespace}/${name}/${version}/terraform-provider-${name}_${version}_SHA256SUMS" \
        --arg shasums_signature_url "${base_url}/${namespace}/${name}/${version}/terraform-provider-${name}_${version}_SHA256SUMS.sig" \
        --argjson signing_keys "${signing_keys}" \
        '{
            "os": $os,
            "arch": $arch,
            "filename": $filename,
            "download_url": $download_url,
            "shasum": $shasum,
            "shasums_url": $shasums_url,
            "shasums_signature_url": $shasums_signature_url,
            "protocols": ["5.0"],
            "signing_keys": $signing_keys
        }' > "${provider_dir}/metadata.json"
    
    echo -e "${GREEN}✅ Created metadata.json${NC}"
    echo ""
}

# Function to create provider index metadata
create_provider_metadata() {
    local namespace=$1
    local name=$2
    local version=$3
    
    echo -e "${BLUE}📋 Creating provider metadata for ${namespace}/${name}...${NC}"
    
    local provider_base_dir="${EXAMPLES_DIR}/providers/${namespace}/${name}"
    
    # Collect available platforms
    local platforms_json="["
    local first=true
    
    for platform in "${PLATFORMS[@]}"; do
        IFS='_' read -r os arch <<< "$platform"
        local platform_dir="${provider_base_dir}/${version}/${os}/${arch}"
        
        if [ -d "$platform_dir" ] && [ -f "${platform_dir}/metadata.json" ]; then
            if [ "$first" = false ]; then
                platforms_json+=","
            fi
            platforms_json+="{\"os\":\"${os}\",\"arch\":\"${arch}\"}"
            first=false
        fi
    done
    platforms_json+="]"
    
    # Create main provider metadata
    cat > "${provider_base_dir}/metadata.json" << EOF
{
  "versions": [
    {
      "version": "${version}",
      "protocols": ["5.0"],
      "platforms": ${platforms_json}
    }
  ]
}
EOF
    
    echo -e "${GREEN}✅ Created provider metadata${NC}"
}

# Check dependencies
echo "🔍 Checking dependencies..."
if ! command -v curl >/dev/null 2>&1; then
    echo -e "${RED}❌ curl not found. Please install curl.${NC}"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo -e "${RED}❌ jq not found. Please install jq.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Dependencies check passed${NC}"
echo ""

# Create base directory
mkdir -p "$EXAMPLES_DIR/providers"

# Download providers
for provider_spec in "${PROVIDERS_LIST[@]}"; do
    IFS=':' read -r namespace name version <<< "$provider_spec"
    
    echo -e "${BLUE}🔽 Processing ${namespace}/${name} v${version}${NC}"
    echo "================================================"
    
    # Track successful downloads
    success_count=0
    
    # Download for each platform
    for platform in "${PLATFORMS[@]}"; do
        IFS='_' read -r os arch <<< "$platform"
        
        if download_provider "$namespace" "$name" "$version" "$os" "$arch"; then
            ((success_count++))
        fi
    done
    
    if [ $success_count -gt 0 ]; then
        create_provider_metadata "$namespace" "$name" "$version"
        echo -e "${GREEN}✅ Successfully downloaded ${success_count} platform(s) for ${namespace}/${name}${NC}"
    else
        echo -e "${RED}❌ No platforms downloaded for ${namespace}/${name}${NC}"
    fi
    
    echo ""
done

echo -e "${GREEN}🎉 Provider download complete!${NC}"
echo ""
echo "📁 Provider binaries downloaded to: ${EXAMPLES_DIR}/providers/"
echo ""
echo "📋 Summary:"
echo "   • Real provider binaries from registry.terraform.io"
echo "   • Platform-specific metadata.json files"
echo "   • Provider index metadata files"

if [ "$USE_S3" = true ]; then
    echo "   • Metadata configured for S3: s3://${S3_BUCKET}"
else
    echo "   • Metadata configured for LocalStack"
fi

echo ""
echo "🔄 Next steps:"
if [ "$USE_S3" = true ]; then
    echo "   1. Upload to S3: ./scripts/upload-to-s3.sh --bucket ${S3_BUCKET} --region ${S3_REGION}"
    echo "   2. Configure your registry to use S3 backend"
    echo "   3. Test Terraform integration"
else
    echo "   1. Upload to LocalStack: ./scripts/manage-localstack.sh upload"
    echo "   2. Test the registry: ./scripts/test-api.sh"
    echo "   3. Test Terraform integration: ./scripts/setup-terraform.sh"
fi 
