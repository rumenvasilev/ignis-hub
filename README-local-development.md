# Local Development Setup

This guide will help you set up a complete local development environment for the Terraform Registry API using Docker Compose with LocalStack for S3 emulation.

## 🚀 Quick Start

### 1. Start the Environment

```bash
# Start all services (LocalStack + Registry)
docker-compose up -d

# Check logs
docker-compose logs -f terraform-registry
```

### 2. Test the API

```bash
# Use the provided test script
./scripts/test-api.sh
```

### 3. Explore the Data

```bash
# List what's in S3
./scripts/manage-localstack.sh list

# Check LocalStack status
./scripts/manage-localstack.sh status
```

## 📁 What Gets Created

The setup automatically creates:

### Sample Modules:
- `terraform-aws-modules/vpc/aws` (versions: 5.0.0, 4.0.2, 3.19.0)
- `hashicorp/consul/aws` (versions: 0.11.0, 0.10.1)

### Sample Providers:
- `hashicorp/aws` (version 5.20.1) - darwin/amd64, linux/amd64, windows/amd64
- `hashicorp/random` (version 3.5.1) - multiple platforms

## 🛠 Services Overview

### LocalStack (Port 4566)
- **Purpose**: Emulates AWS S3 locally
- **Health Check**: `curl http://localhost:4566/_localstack/health`
- **Web UI**: Not included in this basic setup
- **Data Persistence**: Enabled in `./tmp/localstack`

### Terraform Registry (Port 8080)
- **Purpose**: Your Go implementation of the registry API
- **Health Check**: `curl http://localhost:8080/health`
- **Configuration**: Uses environment variables (see docker-compose.yml)
- **Authentication**: Disabled for local development

## 🧪 Testing the API

### Manual Testing

```bash
# 1. Service discovery
curl http://localhost:8080/.well-known/terraform.json

# 2. List module versions
curl http://localhost:8080/v1/modules/terraform-aws-modules/vpc/aws/versions

# 3. Get module download URL
curl -i http://localhost:8080/v1/modules/terraform-aws-modules/vpc/aws/5.0.0/download

# 4. List provider versions
curl http://localhost:8080/v1/providers/hashicorp/aws/versions

# 5. Get provider binary metadata
curl http://localhost:8080/v1/providers/hashicorp/aws/5.20.1/download/linux/amd64
```

### Automated Testing

```bash
# Run comprehensive test suite
./scripts/test-api.sh

# Expected output: All endpoints should return proper JSON responses
```

## 🔧 Management Scripts

### LocalStack Management (`./scripts/manage-localstack.sh`)

```bash
# Show help
./scripts/manage-localstack.sh help

# List all objects in the bucket
./scripts/manage-localstack.sh list

# Check status of LocalStack and bucket
./scripts/manage-localstack.sh status

# Re-upload example data
./scripts/manage-localstack.sh upload

# Clean all data
./scripts/manage-localstack.sh clean

# Download a specific file
./scripts/manage-localstack.sh download registry/modules/terraform-aws-modules/vpc/aws/metadata.json
```

### API Testing (`./scripts/test-api.sh`)

```bash
# Test all endpoints
./scripts/test-api.sh
```

## 🐛 Troubleshooting

### Common Issues

#### 1. LocalStack Not Starting
```bash
# Check if Docker is running
docker ps

# Check LocalStack logs
docker-compose logs localstack

# Restart LocalStack
docker-compose restart localstack
```

#### 2. Registry Can't Connect to S3
```bash
# Check if LocalStack is healthy
curl http://localhost:4566/_localstack/health

# Check registry logs
docker-compose logs terraform-registry

# Restart the registry
docker-compose restart terraform-registry
```

#### 3. No Data in S3
```bash
# Check if initialization completed
docker-compose logs init-localstack

# Manually upload data
./scripts/manage-localstack.sh upload
```

#### 4. API Returns 404s
```bash
# Verify data is in S3
./scripts/manage-localstack.sh list

# Check API configuration
docker-compose logs terraform-registry | grep -i "s3\|bucket"
```

### Debug Commands

```bash
# Check all service status
docker-compose ps

# Follow all logs
docker-compose logs -f

# Restart everything
docker-compose down && docker-compose up -d

# Clean slate restart
docker-compose down -v && docker-compose up -d
```

## 🔄 Development Workflow

### 1. Make Code Changes

```bash
# Edit your Go code
# Example: internal/handlers/handlers.go

# Rebuild and restart
docker-compose build terraform-registry
docker-compose restart terraform-registry
```

### 2. Add New Sample Data

```bash
# Add files to examples/ directory
# Example: examples/modules/myorg/mymodule/aws/metadata.json

# Upload to LocalStack
./scripts/manage-localstack.sh upload

# Test the new endpoint
curl http://localhost:8080/v1/modules/myorg/mymodule/aws/versions
```

### 3. Test Changes

```bash
# Run tests
./scripts/test-api.sh

# Manual testing
curl http://localhost:8080/health
```

## 🌐 Using the Registry with Terraform

To test your registry with actual Terraform:

### 1. Configure Terraform

Create a `~/.terraformrc` file:
```hcl
provider_installation {
  network_mirror {
    url = "http://localhost:8080/v1/providers/"
  }
}
```

### 2. Create a Test Configuration

```hcl
# test.tf
terraform {
  required_providers {
    aws = {
      source  = "localhost:8080/hashicorp/aws"
      version = "5.20.1"
    }
  }
}

# This won't actually work since we have dummy binaries,
# but it will test the registry protocol
```

### 3. Initialize Terraform

```bash
terraform init
# This will attempt to download from your local registry
```

## 📊 Sample Data Structure

The example data follows this S3 structure:

```
terraform-registry/
└── registry/
    ├── modules/
    │   ├── terraform-aws-modules/
    │   │   └── vpc/
    │   │       └── aws/
    │   │           ├── metadata.json
    │   │           └── 5.0.0/
    │   │               └── archive.tar.gz
    │   └── hashicorp/
    │       └── consul/
    │           └── aws/
    │               └── metadata.json
    └── providers/
        └── hashicorp/
            ├── aws/
            │   ├── metadata.json
            │   └── 5.20.1/
            │       ├── darwin/
            │       │   └── amd64/
            │       │       ├── metadata.json
            │       │       └── terraform-provider-aws_5.20.1_darwin_amd64.zip
            │       └── linux/
            │           └── amd64/
            │               ├── metadata.json
            │               └── terraform-provider-aws_5.20.1_linux_amd64.zip
            └── random/
                ├── metadata.json
                └── 3.5.1/
                    └── linux/
                        └── amd64/
                            ├── metadata.json
                            └── terraform-provider-random_3.5.1_linux_amd64.zip
```

## 🎯 Next Steps

1. **Add Real Data**: Replace dummy files with actual Terraform modules and providers
2. **Enable Authentication**: Test with auth enabled in local config
3. **Add More Providers**: Extend the example data with more providers/modules
4. **Performance Testing**: Use tools like `wrk` or `ab` to load test your API
5. **CI/CD Integration**: Set up automated testing in your CI pipeline

## 🔗 Useful URLs

When everything is running:

- **Registry API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health
- **Service Discovery**: http://localhost:8080/.well-known/terraform.json
- **LocalStack**: http://localhost:4566
- **LocalStack Health**: http://localhost:4566/_localstack/health

## 🧹 Cleanup

```bash
# Stop services
docker-compose down

# Remove volumes and data
docker-compose down -v
rm -rf tmp/localstack

# Remove images
docker-compose down --rmi all
```

This local setup gives you a complete, working Terraform Registry that you can use for development, testing, and experimentation! 🎉 
