#!/bin/bash

# LocalStack management script for Terraform Registry
set -e

ENDPOINT="http://localhost:4566"
BUCKET="terraform-registry"
AWS_ARGS="--endpoint-url=$ENDPOINT --region=us-east-1"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test

# Functions
show_help() {
    echo "LocalStack S3 Management Script"
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  list        - List all objects in the bucket"
    echo "  create      - Create the S3 bucket"
    echo "  upload      - Upload example data to S3"
    echo "  download    - Download a specific file"
    echo "  clean       - Clean all data from bucket"
    echo "  status      - Show LocalStack and bucket status"
    echo "  help        - Show this help"
}

check_localstack() {
    if ! curl -s "$ENDPOINT/_localstack/health" > /dev/null; then
        echo "❌ LocalStack is not running or not accessible at $ENDPOINT"
        echo "   Run: docker-compose up -d localstack"
        exit 1
    fi
    echo "✅ LocalStack is running"
}

list_objects() {
    echo "📋 Listing objects in bucket '$BUCKET':"
    aws $AWS_ARGS s3 ls s3://$BUCKET/registry --recursive --human-readable || echo "❌ Failed to list objects"
}

create_bucket() {
    echo "🪣 Creating bucket '$BUCKET':"
    aws $AWS_ARGS s3 mb s3://$BUCKET || echo "❌ Failed to create bucket (might already exist)"
}

upload_data() {
    echo "📤 Uploading example data to S3:"
    if [ -d "examples" ]; then
        aws $AWS_ARGS s3 sync examples s3://$BUCKET/registry --delete
        echo "✅ Upload completed"
    else
        echo "❌ 'examples' directory not found"
        exit 1
    fi
}

download_file() {
    local key="$1"
    if [ -z "$key" ]; then
        echo "Usage: $0 download <s3-key>"
        echo "Example: $0 download registry/modules/terraform-aws-modules/vpc/aws/metadata.json"
        exit 1
    fi
    
    echo "📥 Downloading: $key"
    aws $AWS_ARGS s3 cp "s3://$BUCKET/$key" . || echo "❌ Failed to download file"
}

clean_bucket() {
    echo "🧹 Cleaning bucket '$BUCKET':"
    aws $AWS_ARGS s3 rm s3://$BUCKET --recursive || echo "❌ Failed to clean bucket"
}

show_status() {
    echo "🔍 LocalStack Status:"
    curl -s "$ENDPOINT/_localstack/health" | jq . || echo "❌ Failed to get LocalStack status"
    echo ""
    echo "🪣 Bucket Status:"
    aws $AWS_ARGS s3 ls || echo "❌ Failed to list buckets"
    echo ""
    if aws $AWS_ARGS s3 ls s3://$BUCKET > /dev/null 2>&1; then
        echo "✅ Bucket '$BUCKET' exists"
        object_count=$(aws $AWS_ARGS s3 ls s3://$BUCKET/registry --recursive | wc -l)
        echo "📊 Objects in bucket: $object_count"
    else
        echo "❌ Bucket '$BUCKET' does not exist"
    fi
}

# Main script
case "${1:-help}" in
    list)
        check_localstack
        list_objects
        ;;
    create)
        check_localstack
        create_bucket
        ;;
    upload)
        check_localstack
        create_bucket
        upload_data
        ;;
    download)
        check_localstack
        download_file "$2"
        ;;
    clean)
        check_localstack
        clean_bucket
        ;;
    status)
        check_localstack
        show_status
        ;;
    help|*)
        show_help
        ;;
esac 
