#!/bin/bash

# Verification script for Terraform Registry local development setup
set -e

echo "🔍 Verifying Terraform Registry Local Development Setup"
echo "===================================================="

# Check required files
echo "📁 Checking project structure..."

required_files=(
    "docker-compose.yml"
    "Dockerfile" 
    "config.yaml"
    "config-local.yaml"
    "main.go"
    "go.mod"
    "scripts/test-api.sh"
    "scripts/manage-localstack.sh"
    "examples/modules/terraform-aws-modules/vpc/aws/metadata.json"
    "examples/providers/hashicorp/aws/metadata.json"
    "README-local-development.md"
)

missing_files=()
for file in "${required_files[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ $file (missing)"
        missing_files+=("$file")
    fi
done

# Check directories
echo ""
echo "📂 Checking directory structure..."

required_dirs=(
    "internal/config"
    "internal/models"
    "internal/storage"
    "internal/handlers"
    "internal/middleware"
    "examples/modules"
    "examples/providers"
    "scripts"
)

missing_dirs=()
for dir in "${required_dirs[@]}"; do
    if [ -d "$dir" ]; then
        echo "  ✅ $dir/"
    else
        echo "  ❌ $dir/ (missing)"
        missing_dirs+=("$dir")
    fi
done

# Check Go build
echo ""
echo "🔧 Checking Go build..."
if go build -o terraform-registry-test main.go 2>/dev/null; then
    echo "  ✅ Go build successful"
    rm -f terraform-registry-test
else
    echo "  ❌ Go build failed"
    echo "  Run 'go mod tidy' to fix dependencies"
fi

# Check Docker setup
echo ""
echo "🐳 Checking Docker setup..."
if command -v docker >/dev/null 2>&1; then
    echo "  ✅ Docker is installed"
    if docker info >/dev/null 2>&1; then
        echo "  ✅ Docker daemon is running"
        
        # Test docker-compose
        if docker-compose config >/dev/null 2>&1; then
            echo "  ✅ docker-compose.yml is valid"
        else
            echo "  ❌ docker-compose.yml has issues"
        fi
    else
        echo "  ⚠️  Docker daemon not running (needed for LocalStack)"
        echo "     Start Docker: colima start (if using Colima) or Docker Desktop"
    fi
else
    echo "  ❌ Docker not installed"
fi

# Check sample data
echo ""
echo "📊 Checking sample data..."
module_count=$(find examples/modules -name "metadata.json" 2>/dev/null | wc -l)
provider_count=$(find examples/providers -name "metadata.json" 2>/dev/null | wc -l)
binary_count=$(find examples/providers -name "*.zip" 2>/dev/null | wc -l)

echo "  📦 Module metadata files: $module_count"
echo "  🔌 Provider metadata files: $provider_count"  
echo "  📄 Provider binary files: $binary_count"

# Summary
echo ""
echo "📋 Setup Summary"
echo "================"

if [ ${#missing_files[@]} -eq 0 ] && [ ${#missing_dirs[@]} -eq 0 ]; then
    echo "✅ All required files and directories are present"
else
    echo "❌ Missing components:"
    for file in "${missing_files[@]}"; do
        echo "   - $file"
    done
    for dir in "${missing_dirs[@]}"; do
        echo "   - $dir/"
    done
fi

echo ""
echo "🚀 Ready to Start Development!"
echo "==============================="
echo ""
echo "To get started:"
echo "1. Start Docker (if not running): colima start"
echo "2. Launch the environment: docker-compose up -d"
echo "3. Test the API: ./scripts/test-api.sh"
echo "4. Explore the data: ./scripts/manage-localstack.sh list"
echo ""
echo "📖 Documentation:"
echo "- Local Development: README-local-development.md"
echo "- Full Implementation: README-go-implementation.md"
echo ""
echo "🔗 URLs (when running):"
echo "- Registry API: http://localhost:8080"
echo "- LocalStack: http://localhost:4566"
echo "- Health Check: http://localhost:8080/health" 
