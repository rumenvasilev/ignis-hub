# Terraform Sample Configuration

This directory contains a sample Terraform configuration that demonstrates how to use your local Terraform registry API directly.

## What's Included

- **`main.tf`**: Sample Terraform configuration that creates an S3 bucket using AWS and Random providers from your local registry
- **`../../.terraformrc`**: Terraform CLI configuration for using local registry API
- **`../../scripts/setup-terraform.sh`**: Script to verify your local registry is working

## Prerequisites

1. **Registry running**: Your Terraform registry must be running at `http://localhost:8080`
2. **LocalStack running**: LocalStack S3 must be running at `http://localhost:4566` 
3. **Terraform installed**: Version 1.0 or higher

## Quick Start

### Step 1: Verify Registry
First, verify your local registry is working:

```bash
# From the project root directory
./scripts/setup-terraform.sh
```

This script will:
- Check that your registry is running and healthy
- Test the well-known discovery endpoint
- Verify AWS and Random provider endpoints are working
- Configure Terraform CLI to use your local registry

### Step 2: Initialize Terraform
```bash
cd examples/terraform/

# Set the Terraform config file
export TF_CLI_CONFIG_FILE="$(pwd)/../../.terraformrc"

# Initialize Terraform (will download providers from your local registry)
terraform init
```

### Step 3: Plan and Apply
```bash
# See what Terraform will create
terraform plan

# Apply the configuration (creates real AWS resources)
terraform apply
```

## How It Works

### Direct Registry API Usage
This configuration uses your local registry API directly:

- **Provider Sources**: `localhost:8080/hashicorp/aws` and `localhost:8080/hashicorp/random`
- **Network Mirror**: Points to `http://localhost:8080/v1/providers/`
- **Discovery**: Uses `http://localhost:8080/.well-known/terraform.json`
- **Downloads**: Providers are downloaded during `terraform init` directly from your registry

### Configuration Details

#### `.terraformrc`
```hcl
provider_installation {
  network_mirror {
    url = "http://localhost:8080/v1/providers/"
    include = ["hashicorp/*"]
  }
  
  direct {
    exclude = ["hashicorp/*"]
  }
}

host "localhost:8080" {
  insecure = true
}
```

#### `main.tf` Provider Block
```hcl
required_providers {
  aws = {
    source  = "localhost:8080/hashicorp/aws"
    version = "5.20.1"
  }
  random = {
    source  = "localhost:8080/hashicorp/random"
    version = "3.5.1"
  }
}
```

## Expected Output

During `terraform init`, you should see:

```
Initializing provider plugins...
- Finding localhost:8080/hashicorp/aws versions matching "5.20.1"...
- Finding localhost:8080/hashicorp/random versions matching "3.5.1"...
- Installing localhost:8080/hashicorp/aws v5.20.1...
- Installing localhost:8080/hashicorp/random v3.5.1...

Terraform has been successfully initialized!
```

## Testing with LocalStack

To test against LocalStack instead of real AWS, uncomment the LocalStack configuration in `main.tf`:

```hcl
provider "aws" {
  region = "us-east-1"
  
  # Uncomment these lines for LocalStack testing
  access_key = "test"
  secret_key = "test"
  endpoints {
    s3 = "http://localhost:4566"
  }
}
```

## Verification

### Check Terraform Providers
After `terraform init`, verify Terraform found the providers:

```bash
terraform providers
```

Expected output:
```
Providers required by configuration:
.
├── provider[localhost:8080/hashicorp/aws] 5.20.1
└── provider[localhost:8080/hashicorp/random] 3.5.1
```

### Manual Registry Testing
Test the registry endpoints directly:

```bash
# Health check
curl http://localhost:8080/health

# Well-known discovery
curl http://localhost:8080/.well-known/terraform.json

# List AWS provider versions
curl http://localhost:8080/v1/providers/hashicorp/aws/versions

# Get AWS provider download info
curl http://localhost:8080/v1/providers/hashicorp/aws/5.20.1/download/darwin/amd64
```

## Troubleshooting

### "Registry not running"
Ensure your registry is started:
```bash
REGISTRY_AWS_REGION=us-east-1 \
REGISTRY_AWS_S3_BUCKET=terraform-registry \
REGISTRY_AWS_S3_PREFIX=registry \
REGISTRY_AWS_ENDPOINT=http://localhost:4566 \
REGISTRY_AWS_ACCESS_KEY_ID=test \
REGISTRY_AWS_SECRET_ACCESS_KEY=test \
REGISTRY_AUTH_ENABLED=false \
go run main.go
```

### "Could not connect to localhost:8080"
1. Check registry health: `curl http://localhost:8080/health`
2. Verify LocalStack is running: `curl http://localhost:4566/_localstack/health`
3. Check that `.terraformrc` includes `insecure = true` for localhost:8080

### "Provider not found"
1. Verify provider exists: `curl http://localhost:8080/v1/providers/hashicorp/aws/versions`
2. Check provider binary exists in S3: `aws --endpoint-url=http://localhost:4566 s3 ls s3://terraform-registry/registry/providers/hashicorp/aws/`
3. Re-run setup: `./scripts/setup-terraform.sh`

### "Authentication errors" 
Make sure `REGISTRY_AUTH_ENABLED=false` when running the registry for testing.

## Advantages of Direct API Usage

- ✅ **Real-time**: Always gets latest providers from your registry
- ✅ **No local storage**: No need to manage filesystem mirrors
- ✅ **Registry features**: Full access to all registry API features
- ✅ **Simpler**: Direct communication with your registry API

## Cleanup

```bash
# Destroy Terraform resources
terraform destroy

# Remove Terraform state files
rm -rf .terraform .terraform.lock.hcl terraform.tfstate*
``` 
