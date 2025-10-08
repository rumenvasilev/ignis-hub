#!/bin/bash

# Test script for Terraform Registry API
set -e

BASE_URL="http://localhost:8080"

echo "🧪 Testing Terraform Registry API at $BASE_URL"
echo "================================================"

# Test 1: Health check
echo "1. Health Check:"
curl -s "$BASE_URL/health" | jq . || echo "❌ Health check failed"
echo ""

# Test 2: Well-known endpoint
echo "2. Well-known endpoint:"
curl -s "$BASE_URL/.well-known/terraform.json" | jq . || echo "❌ Well-known endpoint failed"
echo ""

# Test 3: List module versions
echo "3. List module versions (terraform-aws-modules/vpc/aws):"
curl -s "$BASE_URL/v1/modules/terraform-aws-modules/vpc/aws/versions" | jq . || echo "❌ Module versions failed"
echo ""

# Test 4: Module download URL
echo "4. Get module download URL (terraform-aws-modules/vpc/aws/5.0.0):"
response=$(curl -s -i "$BASE_URL/v1/modules/terraform-aws-modules/vpc/aws/5.0.0/download")
echo "$response" | grep -E "(HTTP|X-Terraform-Get)" || echo "❌ Module download failed"
echo ""

# Test 5: List provider versions
echo "5. List provider versions (hashicorp/aws):"
curl -s "$BASE_URL/v1/providers/hashicorp/aws/versions" | jq . || echo "❌ Provider versions failed"
echo ""

# Test 6: Get provider binary info
echo "6. Get provider binary (hashicorp/aws/5.20.1/linux/amd64):"
curl -s "$BASE_URL/v1/providers/hashicorp/aws/5.20.1/download/linux/amd64" | jq . || echo "❌ Provider binary failed"
echo ""

# Test 7: Root endpoint
echo "7. Root endpoint:"
curl -s "$BASE_URL/" | jq . || echo "❌ Root endpoint failed"
echo ""

echo "✅ All tests completed!"
echo ""
echo "📋 Available endpoints:"
echo "  - Health: $BASE_URL/health"
echo "  - Well-known: $BASE_URL/.well-known/terraform.json" 
echo "  - Modules: $BASE_URL/v1/modules/{namespace}/{name}/{system}/versions"
echo "  - Providers: $BASE_URL/v1/providers/{namespace}/{type}/versions"
echo ""
echo "🔍 To test with authentication (if enabled):"
echo "  curl -H 'Authorization: Bearer your-token' $BASE_URL/.well-known/terraform.json" 
