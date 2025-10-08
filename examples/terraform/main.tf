terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      #version = "5.20.1"
      version = "5.100.0"
    }
    fastssm = {
      source  = "rumenvasilev/fastssm"
      version = "0.1.6"
    }
  }
}

# Configure the AWS Provider
provider "aws" {
  region = "eu-west-1"
  
  # For testing with LocalStack
  # access_key = "test"
  # secret_key = "test"
  # skip_credentials_validation = true
  # skip_requesting_account_id = true
  # s3_use_path_style = true  # Force path-style URLs for LocalStack
  # endpoints {
  #   s3  = "http://localhost:4566"
  #   sts = "http://localhost:4566"
  # }
}

# Example AWS resource - S3 bucket with fixed name
resource "aws_s3_bucket" "example" {
  bucket = "my-terraform-registry-test-bucket"
}

resource "aws_s3_bucket_versioning" "example" {
  bucket = aws_s3_bucket.example.id
  versioning_configuration {
    status = "Enabled"
  }
}

# Output the bucket name
output "bucket_name" {
  description = "Name of the S3 bucket"
  value       = aws_s3_bucket.example.bucket
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.example.arn
} 
