output "bucket_id" {
  description = "ID of the S3 bucket"
  value       = aws_s3_bucket.registry.id
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.registry.arn
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = aws_s3_bucket.registry.bucket_domain_name
}

output "bucket_regional_domain_name" {
  description = "Regional domain name of the S3 bucket"
  value       = aws_s3_bucket.registry.bucket_regional_domain_name
}

output "vpc_endpoint_id" {
  description = "ID of the VPC endpoint for S3 (either created or existing)"
  value       = local.vpc_endpoint_id
}

output "vpc_endpoint_prefix_list_id" {
  description = "Prefix list ID of the VPC endpoint (can be used in security group rules) - only available if endpoint was created"
  value       = var.create_vpc_endpoint ? aws_vpc_endpoint.s3[0].prefix_list_id : null
}

output "vpc_endpoint_created" {
  description = "Whether a new VPC endpoint was created by this module"
  value       = var.create_vpc_endpoint
}

output "account_id" {
  description = "AWS account ID where resources are created"
  value       = data.aws_caller_identity.current.account_id
}

output "region" {
  description = "AWS region where resources are created"
  value       = data.aws_region.current.name
}
