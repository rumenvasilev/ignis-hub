variable "name" {
  description = "Name for the resource"
  type        = string
  default     = "test"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

output "greeting" {
  value = "Hello, ${var.name} in ${var.environment}!"
}

output "timestamp" {
  value = timestamp()
}

