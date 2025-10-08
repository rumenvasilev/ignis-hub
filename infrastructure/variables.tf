variable "aws_region" {
  description = "AWS region where resources will be created"
  type        = string
  default     = "us-east-1"
}

variable "vpc_id" {
  description = "VPC ID where the Kubernetes cluster is running (required only if creating a new VPC endpoint)"
  type        = string
  default     = null

  validation {
    condition     = !var.create_vpc_endpoint || var.vpc_id != null
    error_message = "vpc_id must be provided when create_vpc_endpoint is true."
  }
}

variable "route_table_ids" {
  description = "List of route table IDs to associate with the S3 VPC endpoint (required only if creating a new VPC endpoint)"
  type        = list(string)
  default     = []

  validation {
    condition     = !var.create_vpc_endpoint || length(var.route_table_ids) > 0
    error_message = "route_table_ids must contain at least one route table ID when create_vpc_endpoint is true."
  }
}

variable "create_vpc_endpoint" {
  description = "Whether to create a new S3 VPC endpoint. Set to false if you already have one configured."
  type        = bool
  default     = true
}

variable "existing_vpc_endpoint_id" {
  description = "ID of an existing S3 VPC endpoint to use (required if create_vpc_endpoint is false)"
  type        = string
  default     = null

  validation {
    condition     = var.create_vpc_endpoint || var.existing_vpc_endpoint_id != null
    error_message = "existing_vpc_endpoint_id must be provided when create_vpc_endpoint is false."
  }
}

variable "bucket_name_prefix" {
  description = "Name prefix for the S3 bucket (must be globally unique)"
  type        = string
}

variable "enable_versioning" {
  description = "Enable versioning for the S3 bucket"
  type        = bool
  default     = true
}

variable "enable_lifecycle_rules" {
  description = "Enable lifecycle rules for the S3 bucket"
  type        = bool
  default     = true
}

variable "noncurrent_version_expiration_days" {
  description = "Number of days after which to expire noncurrent object versions"
  type        = number
  default     = 90
}

variable "common_tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default = {
    Project     = "terraform-registry"
    ManagedBy   = "terraform"
    Environment = "production"
  }
}
