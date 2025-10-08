# Usage Examples

Quick reference for different deployment scenarios.

## Scenario 1: Create New VPC Endpoint (Default)

Use this when you **don't have** an existing S3 VPC endpoint in your VPC.

### terraform.tfvars
```hcl
aws_region          = "us-east-1"
create_vpc_endpoint = true
vpc_id              = "vpc-0a1b2c3d4e5f6g7h8"
route_table_ids     = [
  "rtb-0123456789abcdef0",
  "rtb-0fedcba9876543210"
]
bucket_name_prefix = "my-terraform-registry-prod"

common_tags = {
  Project     = "terraform-registry"
  Environment = "production"
  Team        = "platform"
}
```

### Commands
```bash
cd infrastructure/
terraform init
terraform plan
terraform apply
```

### What Gets Created
- ✅ New S3 Gateway VPC Endpoint
- ✅ Private S3 bucket with VPC-only access
- ✅ Bucket policy restricting access to the VPC endpoint

---

## Scenario 2: Use Existing VPC Endpoint

Use this when you **already have** an S3 VPC endpoint configured in your VPC.

### Step 1: Find Your Existing VPC Endpoint

```bash
# Find existing S3 VPC endpoints in your VPC
aws ec2 describe-vpc-endpoints \
  --filters "Name=vpc-id,Values=vpc-YOUR-VPC-ID" \
            "Name=service-name,Values=com.amazonaws.us-east-1.s3" \
  --query "VpcEndpoints[?State=='available'].[VpcEndpointId,RouteTableIds]" \
  --output table
```

Example output:
```
vpce-0abc123def456789  | ["rtb-0123", "rtb-0456"]
```

### Step 2: Configure terraform.tfvars

```hcl
aws_region               = "us-east-1"
create_vpc_endpoint      = false
existing_vpc_endpoint_id = "vpce-0abc123def456789"
bucket_name_prefix       = "my-terraform-registry-prod"

common_tags = {
  Project     = "terraform-registry"
  Environment = "production"
  Team        = "platform"
}
```

### Step 3: Apply

```bash
cd infrastructure/
terraform init
terraform plan
terraform apply
```

### What Gets Created
- ✅ Private S3 bucket with VPC-only access
- ✅ Bucket policy restricting access to your existing VPC endpoint
- ⏭️ VPC endpoint is skipped (using existing one)

---

## Scenario 3: EKS Cluster Setup

For EKS clusters, use this workflow to find your configuration:

### Step 1: Get EKS VPC Information

```bash
# Set your cluster name
CLUSTER_NAME="your-eks-cluster"

# Get VPC ID
VPC_ID=$(aws eks describe-cluster --name $CLUSTER_NAME \
  --query "cluster.resourcesVpcConfig.vpcId" --output text)
echo "VPC ID: $VPC_ID"

# Check for existing S3 VPC endpoint
aws ec2 describe-vpc-endpoints \
  --filters "Name=vpc-id,Values=$VPC_ID" \
            "Name=service-name,Values=com.amazonaws.$(aws configure get region).s3" \
  --query "VpcEndpoints[?State=='available'].VpcEndpointId" \
  --output text
```

### Step 2a: If NO Existing Endpoint

```bash
# Get route table IDs for your EKS subnets
aws eks describe-cluster --name $CLUSTER_NAME \
  --query "cluster.resourcesVpcConfig.subnetIds" --output text | \
  xargs -I {} aws ec2 describe-route-tables \
  --filters "Name=association.subnet-id,Values={}" \
  --query "RouteTables[].RouteTableId" --output text
```

Create `terraform.tfvars`:
```hcl
aws_region          = "us-east-1"
create_vpc_endpoint = true
vpc_id              = "vpc-YOUR-VPC-ID"  # from Step 1
route_table_ids     = ["rtb-XXX", "rtb-YYY"]  # from this step
bucket_name_prefix  = "my-terraform-registry-prod"
```

### Step 2b: If Existing Endpoint Found

Create `terraform.tfvars`:
```hcl
aws_region               = "us-east-1"
create_vpc_endpoint      = false
existing_vpc_endpoint_id = "vpce-XXXXX"  # from Step 1
bucket_name_prefix       = "my-terraform-registry-prod"
```

---

## Validation

The configuration includes built-in validation:

### ✅ Valid Configurations

```hcl
# Option 1: Creating new endpoint
create_vpc_endpoint = true
vpc_id              = "vpc-123"
route_table_ids     = ["rtb-123"]
```

```hcl
# Option 2: Using existing endpoint
create_vpc_endpoint      = false
existing_vpc_endpoint_id = "vpce-123"
```

### ❌ Invalid Configurations

```hcl
# ERROR: Missing VPC endpoint ID when not creating
create_vpc_endpoint = false
# existing_vpc_endpoint_id not set
```

```hcl
# ERROR: Missing VPC ID when creating endpoint
create_vpc_endpoint = true
# vpc_id not set
```

```hcl
# ERROR: Missing route table IDs when creating endpoint
create_vpc_endpoint = true
vpc_id              = "vpc-123"
route_table_ids     = []  # Empty list!
```

---

## Testing Your Configuration

After applying, test access from a pod in your Kubernetes cluster:

```bash
# Deploy a test pod
kubectl run aws-test -it --rm --restart=Never \
  --image=amazon/aws-cli -- \
  s3 ls s3://your-bucket-name

# If it works, you should see the bucket contents (or empty list)
# If it fails with "Access Denied", check:
# 1. The VPC endpoint is associated with your pod's subnet route tables
# 2. The bucket policy includes the correct VPC endpoint ID
```

---

## Common Issues and Solutions

### Issue: "existing_vpc_endpoint_id must be provided"

**Problem**: You set `create_vpc_endpoint = false` but didn't provide an endpoint ID.

**Solution**: Add `existing_vpc_endpoint_id = "vpce-XXXXX"` to your tfvars.

### Issue: "vpc_id must be provided"

**Problem**: You set `create_vpc_endpoint = true` but didn't provide a VPC ID.

**Solution**: Add `vpc_id = "vpc-XXXXX"` to your tfvars.

### Issue: "route_table_ids must contain at least one route table"

**Problem**: You set `create_vpc_endpoint = true` but didn't provide any route table IDs.

**Solution**: Add `route_table_ids = ["rtb-XXX", "rtb-YYY"]` to your tfvars.

### Issue: "Access Denied" from Kubernetes pods

**Problem**: Pods can't access the S3 bucket.

**Possible causes**:
1. VPC endpoint not associated with pod subnets' route tables
2. Wrong VPC endpoint ID in configuration
3. Pods running in different VPC

**Solution**:
```bash
# Verify endpoint is associated with correct route tables
aws ec2 describe-vpc-endpoints --vpc-endpoint-ids vpce-YOUR-ENDPOINT-ID

# Check which route tables your pods use
kubectl get nodes -o json | \
  jq -r '.items[].spec.providerID' | \
  sed 's/.*\///' | \
  xargs -I {} aws ec2 describe-instances --instance-ids {} \
  --query 'Reservations[].Instances[].[SubnetId]' --output text | \
  xargs -I {} aws ec2 describe-route-tables \
  --filters "Name=association.subnet-id,Values={}"
```

---

## Output Reference

After `terraform apply`, you'll get these outputs:

```
bucket_id                     = "my-terraform-registry-prod"
bucket_arn                    = "arn:aws:s3:::my-terraform-registry-prod"
bucket_domain_name            = "my-terraform-registry-prod.s3.amazonaws.com"
bucket_regional_domain_name   = "my-terraform-registry-prod.s3.us-east-1.amazonaws.com"
vpc_endpoint_id               = "vpce-0abc123def456789"
vpc_endpoint_created          = true  # or false if using existing
vpc_endpoint_prefix_list_id   = "pl-63a5400a"  # or null if using existing
account_id                    = "123456789012"
region                        = "us-east-1"
```

Use these outputs to:
- Configure your application (`bucket_id`, `region`)
- Update security group rules (`vpc_endpoint_prefix_list_id`)
- Verify deployment (`account_id`, `vpc_endpoint_created`)
