# Infrastructure Setup for Terraform Registry in AWS

This Terraform configuration creates a private S3 bucket with VPC endpoint access for hosting a Terraform provider registry. The bucket is configured to be accessible only from within your VPC, making it ideal for Kubernetes workloads without requiring complex IAM permission management.

## Architecture

```
┌─────────────────────────────────────┐
│          Your VPC                   │
│                                     │
│  ┌──────────────────────┐          │
│  │  Kubernetes Cluster  │          │
│  │                      │          │
│  │  ┌────────────────┐ │          │
│  │  │  Pods          │ │          │
│  │  │  (Download     │ │          │
│  │  │   Providers)   │ │          │
│  │  └────────────────┘ │          │
│  └──────────────────────┘          │
│            │                        │
│            │ Private Access         │
│            ▼                        │
│  ┌──────────────────────┐          │
│  │  S3 VPC Endpoint     │          │
│  │  (Gateway Type)      │          │
│  └──────────────────────┘          │
└─────────────│───────────────────────┘
              │
              │ Private Route
              ▼
     ┌──────────────────┐
     │  S3 Bucket       │
     │  (Private)       │
     │  • No Public     │
     │    Access        │
     │  • VPC-Only      │
     │    Policy        │
     └──────────────────┘
```

## Features

- **Private S3 Bucket**: Fully private bucket with all public access blocked
- **VPC Endpoint Access**: S3 Gateway VPC endpoint for private connectivity
- **Restrictive Bucket Policy**: Access only allowed through the VPC endpoint
- **Encryption**: Server-side encryption enabled (AES256)
- **Versioning**: Optional versioning for object history
- **Lifecycle Management**: Automatic cleanup of old versions and incomplete uploads
- **TLS Enforcement**: Deny all non-HTTPS requests

## Prerequisites

1. **Existing VPC**: You must have a VPC where your Kubernetes cluster runs
2. **VPC Endpoint**: Either have an existing S3 VPC endpoint ID, or know your route table IDs to create a new one
3. **Terraform**: Version 1.0 or higher
4. **AWS Credentials**: Configured with appropriate permissions

## Quick Start

> 💡 **Pro Tip**: See [USAGE-EXAMPLES.md](USAGE-EXAMPLES.md) for complete configuration examples and EKS-specific workflows.

### Step 1: Configure Variables

Copy the example variables file and customize it:

```bash
cd infrastructure/
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your values:

**Option 1: Create a new VPC endpoint** (if you don't have one already)

```hcl
aws_region          = "us-east-1"
create_vpc_endpoint = true
vpc_id              = "vpc-YOUR-VPC-ID"
route_table_ids     = ["rtb-YOUR-RT-ID-1", "rtb-YOUR-RT-ID-2"]
bucket_name_prefix  = "your-unique-bucket-name"
```

**Option 2: Use an existing VPC endpoint** (if one already exists in your VPC)

```hcl
aws_region               = "us-east-1"
create_vpc_endpoint      = false
existing_vpc_endpoint_id = "vpce-YOUR-EXISTING-ENDPOINT-ID"
bucket_name_prefix       = "your-unique-bucket-name"
```

**Important**: The bucket name must be globally unique across all AWS accounts.

### Step 2: Initialize Terraform

```bash
terraform init
```

### Step 3: Plan and Apply

```bash
# Review the changes
terraform plan

# Apply the configuration
terraform apply
```

## Finding Your VPC Endpoint and Route Table IDs

### Check for Existing S3 VPC Endpoint

First, check if you already have an S3 VPC endpoint:

```bash
# Using AWS CLI
aws ec2 describe-vpc-endpoints \
  --filters "Name=vpc-id,Values=vpc-YOUR-VPC-ID" "Name=service-name,Values=com.amazonaws.REGION.s3" \
  --query "VpcEndpoints[*].[VpcEndpointId,State,RouteTableIds]" \
  --output table
```

If you see an endpoint with `State=available`, you can use it with `create_vpc_endpoint = false`.

### Finding VPC and Route Table IDs (if creating new endpoint)

#### Using AWS Console

1. **VPC ID**: Go to VPC Dashboard → Your VPCs → Find your K8s VPC
2. **Route Tables**: Go to VPC Dashboard → Route Tables → Filter by your VPC ID
3. **VPC Endpoints**: Go to VPC Dashboard → Endpoints → Check for existing S3 endpoints

#### Using AWS CLI

```bash
# Find your VPC
aws ec2 describe-vpcs --filters "Name=tag:Name,Values=*kubernetes*"

# Find route tables for your VPC
aws ec2 describe-route-tables --filters "Name=vpc-id,Values=vpc-YOUR-VPC-ID"
```

#### Using kubectl (for EKS)

```bash
# Get VPC ID from EKS cluster
aws eks describe-cluster --name your-cluster-name --query "cluster.resourcesVpcConfig.vpcId"

# Get subnets used by your cluster
aws eks describe-cluster --name your-cluster-name --query "cluster.resourcesVpcConfig.subnetIds"

# Get route tables for those subnets
aws ec2 describe-route-tables --filters "Name=association.subnet-id,Values=subnet-ID"
```

## Configuration Options

### Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `aws_region` | AWS region for resources | No | `us-east-1` |
| `create_vpc_endpoint` | Create new VPC endpoint or use existing | No | `true` |
| `existing_vpc_endpoint_id` | Existing VPC endpoint ID (if not creating) | Conditional* | `null` |
| `vpc_id` | VPC ID where K8s runs (if creating endpoint) | Conditional* | `null` |
| `route_table_ids` | List of route table IDs (if creating endpoint) | Conditional* | `[]` |
| `bucket_name_prefix` | S3 bucket name (globally unique) | Yes | - |
| `enable_versioning` | Enable S3 versioning | No | `true` |
| `enable_lifecycle_rules` | Enable lifecycle policies | No | `true` |
| `noncurrent_version_expiration_days` | Days to keep old versions | No | `90` |
| `common_tags` | Tags to apply to all resources | No | See `variables.tf` |

\* Conditional requirements:
- If `create_vpc_endpoint = true`: `vpc_id` and `route_table_ids` are required
- If `create_vpc_endpoint = false`: `existing_vpc_endpoint_id` is required

### Outputs

After applying, you'll get these outputs:

- `bucket_id`: S3 bucket ID
- `bucket_arn`: S3 bucket ARN
- `bucket_domain_name`: S3 bucket domain name
- `vpc_endpoint_id`: VPC endpoint ID (created or existing)
- `vpc_endpoint_created`: Boolean indicating if a new endpoint was created
- `vpc_endpoint_prefix_list_id`: Prefix list for security group rules (null if using existing endpoint)
- `account_id`: AWS account ID
- `region`: AWS region

## Using the Bucket with Your Registry

After the infrastructure is created, configure your registry application to use the bucket:

```bash
# Using environment variables
export REGISTRY_AWS_REGION="us-east-1"
export REGISTRY_AWS_S3_BUCKET="your-bucket-name"
export REGISTRY_AWS_S3_PREFIX="registry"

# Run your registry
go run main.go
```

For Kubernetes deployments, add these to your pod spec:

```yaml
env:
  - name: REGISTRY_AWS_REGION
    value: "us-east-1"
  - name: REGISTRY_AWS_S3_BUCKET
    value: "your-bucket-name"
  - name: REGISTRY_AWS_S3_PREFIX
    value: "registry"
```

## Security Considerations

### No IAM Credentials Needed (Recommended)

Since your pods run in the same VPC and use the VPC endpoint, **no IAM credentials are needed** for read access. The bucket policy allows access from the VPC endpoint to any principal.

However, for production deployments, you may want to add additional security layers:

### Option 1: IRSA (IAM Roles for Service Accounts) - EKS

For write operations or additional security, use IRSA:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: terraform-registry
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/terraform-registry-role
```

### Option 2: IAM Instance Profile

Attach an IAM role to your K8s nodes:

```hcl
# Add to main.tf if needed
resource "aws_iam_role_policy" "registry_access" {
  name = "terraform-registry-s3-access"
  role = var.k8s_node_role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.registry.arn,
          "${aws_s3_bucket.registry.arn}/*"
        ]
      }
    ]
  })
}
```

### Network Considerations

The S3 VPC endpoint ensures:
- Traffic stays within AWS network (never goes to internet)
- No data transfer charges between VPC and S3
- Lower latency
- Better security posture

## Troubleshooting

### "Access Denied" errors

1. **Check VPC endpoint**: Ensure your pods' route tables include the VPC endpoint
   ```bash
   aws ec2 describe-route-tables --route-table-ids rtb-YOUR-RT-ID
   ```

2. **Verify endpoint policy**: Check the VPC endpoint is associated with correct route tables
   ```bash
   aws ec2 describe-vpc-endpoints --vpc-endpoint-ids vpce-YOUR-ENDPOINT-ID
   ```

3. **Test from pod**:
   ```bash
   kubectl run -it --rm aws-test --image=amazon/aws-cli -- s3 ls s3://your-bucket-name
   ```

### "No route to host" errors

If you created a new VPC endpoint, the route tables might not be correctly associated. Update `route_table_ids` in your `terraform.tfvars`.

If you're using an existing VPC endpoint, verify it's associated with the correct route tables:
```bash
aws ec2 describe-vpc-endpoints --vpc-endpoint-ids vpce-YOUR-ENDPOINT-ID
```

### Bucket name already exists

S3 bucket names must be globally unique. Choose a different `bucket_name_prefix`.

## Cleanup

To destroy all resources:

```bash
# WARNING: This will delete the bucket and all its contents!
terraform destroy
```

If you have objects in the bucket, you may need to empty it first:

```bash
aws s3 rm s3://your-bucket-name --recursive
terraform destroy
```

## Advanced Configuration

### Custom Bucket Policy

To modify the bucket policy (e.g., add specific IAM principals), edit the `aws_s3_bucket_policy` resource in `main.tf`.

### Multiple VPCs

To allow access from multiple VPCs, you'll need to create VPC endpoints in each VPC and update the bucket policy's condition to include multiple VPCe IDs.

You can do this by creating a `locals.tf` file and modifying the bucket policy:

```hcl
# locals.tf
locals {
  allowed_vpc_endpoints = [
    local.vpc_endpoint_id,
    "vpce-additional-endpoint-id-1",
    "vpce-additional-endpoint-id-2"
  ]
}
```

Then modify the bucket policy condition in `main.tf` to use the list:
```hcl
Condition = {
  StringEquals = {
    "aws:SourceVpce" = local.allowed_vpc_endpoints
  }
}
```

### Cross-Region Access

For cross-region access, create VPC endpoints in each region and update the bucket policy accordingly.

## Cost Estimation

### Monthly Costs (example)

- S3 Storage (100 GB): ~$2.30/month
- S3 Requests (1M GET): ~$0.40/month
- VPC Endpoint (Gateway): **FREE**
- Data Transfer (VPC to S3): **FREE**

**Total**: ~$3/month (for 100GB storage with 1M requests)

Gateway VPC endpoints for S3 are free, unlike Interface endpoints which have hourly charges.

## References

- [AWS S3 VPC Endpoints](https://docs.aws.amazon.com/vpc/latest/privatelink/vpc-endpoints-s3.html)
- [S3 Bucket Policies](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucket-policies.html)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
