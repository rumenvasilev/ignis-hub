terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
  
  default_tags {
    tags = var.common_tags
  }
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# VPC endpoint for S3 (optional - only created if create_vpc_endpoint is true)
resource "aws_vpc_endpoint" "s3" {
  count = var.create_vpc_endpoint ? 1 : 0

  vpc_id            = var.vpc_id
  service_name      = "com.amazonaws.${data.aws_region.current.name}.s3"
  vpc_endpoint_type = "Gateway"
  
  route_table_ids = var.route_table_ids

  tags = merge(
    var.common_tags,
    {
      Name = "${var.bucket_name_prefix}-s3-endpoint"
    }
  )
}

# Local value to use either the created or existing VPC endpoint
locals {
  vpc_endpoint_id = var.create_vpc_endpoint ? aws_vpc_endpoint.s3[0].id : var.existing_vpc_endpoint_id
}

# S3 bucket for Terraform providers
resource "aws_s3_bucket" "registry" {
  bucket = var.bucket_name_prefix

  tags = merge(
    var.common_tags,
    {
      Name        = var.bucket_name_prefix
      Description = "Private S3 bucket for Terraform registry"
    }
  )
}

# Block all public access
resource "aws_s3_bucket_public_access_block" "registry" {
  bucket = aws_s3_bucket.registry.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Enable versioning
resource "aws_s3_bucket_versioning" "registry" {
  bucket = aws_s3_bucket.registry.id
  
  versioning_configuration {
    status = var.enable_versioning ? "Enabled" : "Disabled"
  }
}

# Enable server-side encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "registry" {
  bucket = aws_s3_bucket.registry.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

# Lifecycle rules
resource "aws_s3_bucket_lifecycle_configuration" "registry" {
  count  = var.enable_lifecycle_rules ? 1 : 0
  bucket = aws_s3_bucket.registry.id

  rule {
    id     = "expire-old-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = var.noncurrent_version_expiration_days
    }
  }

  rule {
    id     = "abort-incomplete-uploads"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Bucket policy to restrict access to VPC endpoint
resource "aws_s3_bucket_policy" "registry" {
  bucket = aws_s3_bucket.registry.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowReadFromVPC"
        Effect = "Allow"
        Principal = "*"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.registry.arn,
          "${aws_s3_bucket.registry.arn}/*"
        ]
        Condition = {
          StringEquals = {
            "aws:SourceVpce" = local.vpc_endpoint_id
          }
        }
      },
      {
        Sid    = "AllowWriteFromVPC"
        Effect = "Allow"
        Principal = "*"
        Action = [
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "${aws_s3_bucket.registry.arn}/*"
        ]
        Condition = {
          StringEquals = {
            "aws:SourceVpce" = local.vpc_endpoint_id
          }
        }
      },
      {
        Sid    = "DenyInsecureTransport"
        Effect = "Deny"
        Principal = "*"
        Action = "s3:*"
        Resource = [
          aws_s3_bucket.registry.arn,
          "${aws_s3_bucket.registry.arn}/*"
        ]
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      }
    ]
  })

  depends_on = [
    aws_s3_bucket_public_access_block.registry
  ]
}
