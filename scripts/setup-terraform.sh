#!/bin/bash

set -e

echo "🛠️  Setting up Terraform with Local Registry API"
echo "==============================================="

# Configuration
REGISTRY_URL="http://localhost:8080"
PROVIDERS_DIR="/tmp/terraform-providers"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) 
        # For now, use amd64 since our sample data has more amd64 binaries
        echo -e "${YELLOW}⚠️  ARM64 detected, but using amd64 binaries for compatibility with sample data${NC}"
        ARCH="amd64" 
        ;;
    *) echo -e "${RED}❌ Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    *) echo -e "${RED}❌ Unsupported OS: $OS${NC}"; exit 1 ;;
esac

echo -e "${BLUE}🔧 Platform: ${OS}/${ARCH}${NC}"
echo -e "${BLUE}🔧 Using filesystem mirror to work around Terraform HTTPS requirement${NC}"

# Check if registry is running
echo ""
echo "🏥 Checking registry health..."
if ! curl -s "${REGISTRY_URL}/health" > /dev/null; then
    echo -e "${RED}❌ Registry not running at $REGISTRY_URL${NC}"
    echo "   Please start the registry first"
    exit 1
fi
echo -e "${GREEN}✅ Registry is running${NC}"

# Check well-known endpoint
echo ""
echo "🔍 Testing service discovery endpoint..."
discovery_response=$(curl -s "${REGISTRY_URL}/.well-known/terraform.json")
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Service discovery working${NC}"
    echo "   Response: $discovery_response"
else
    echo -e "${RED}❌ Service discovery failed${NC}"
    exit 1
fi

# Function to download and setup provider
setup_provider() {
    local namespace=$1
    local type=$2
    local version=$3
    
    echo -e "${YELLOW}📦 Setting up ${namespace}/${type} v${version}...${NC}"
    
    # Get provider download info
    local download_url="${REGISTRY_URL}/v1/providers/${namespace}/${type}/${version}/download/${OS}/${ARCH}"
    local response=$(curl -s "$download_url")
    
    if [ $? -ne 0 ] || [ -z "$response" ]; then
        echo -e "${RED}❌ Failed to get download info for ${namespace}/${type}${NC}"
        return 1
    fi
    
    # Extract download URL and filename
    local binary_url=$(echo "$response" | grep -o '"download_url":"[^"]*"' | cut -d'"' -f4)
    local filename=$(echo "$response" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$binary_url" ] || [ -z "$filename" ]; then
        echo -e "${RED}❌ Could not extract download information${NC}"
        return 1
    fi
    
    echo "📁 Download URL: $binary_url"
    
    # Create provider directory structure for localhost:8080 host
    local provider_dir="${PROVIDERS_DIR}/localhost:8080/${namespace}/${type}/${version}/${OS}_${ARCH}"
    mkdir -p "$provider_dir"
    
    # Download the provider
    echo "⬇️  Downloading..."
    if curl -L -s "$binary_url" -o "${provider_dir}/${filename}"; then
        echo -e "${GREEN}✅ Downloaded ${filename}${NC}"
    else
        echo -e "${RED}❌ Failed to download provider${NC}"
        return 1
    fi
    
    # Extract if ZIP
    if [[ "$filename" == *.zip ]]; then
        echo "📂 Extracting..."
        cd "$provider_dir"
        if command -v unzip >/dev/null 2>&1; then
            unzip -q "$filename" && rm "$filename"
            chmod +x terraform-provider-*
            echo -e "${GREEN}✅ Extracted and made executable${NC}"
        else
            echo -e "${RED}❌ unzip command not found${NC}"
            return 1
        fi
    fi
}

# Create providers directory
echo ""
echo "📁 Creating providers directory..."
mkdir -p "$PROVIDERS_DIR"

# Setup providers
echo ""
echo "📦 Downloading providers from local registry..."

setup_provider "hashicorp" "aws" "5.20.1"
setup_provider "hashicorp" "random" "3.5.1"

# Set up Terraform configuration
echo ""
echo "⚙️  Configuring Terraform..."

export TF_CLI_CONFIG_FILE="$(pwd)/.terraformrc"
echo "Set TF_CLI_CONFIG_FILE=$TF_CLI_CONFIG_FILE"

echo ""
echo -e "${GREEN}🎉 Setup complete!${NC}"
echo ""
echo "📋 Your Terraform configuration is ready:"
echo "   • Registry URL: $REGISTRY_URL"
echo "   • Provider Sources: localhost:8080/hashicorp/aws, localhost:8080/hashicorp/random"
echo "   • Filesystem Mirror: $PROVIDERS_DIR"
echo ""
echo "📋 Next steps:"
echo "   1. cd examples/terraform/"
echo "   2. export TF_CLI_CONFIG_FILE='$(pwd)/.terraformrc'"
echo "   3. terraform init"
echo "   4. terraform plan"
echo ""
echo "🔍 Providers downloaded to: $PROVIDERS_DIR/localhost:8080/hashicorp/" 
