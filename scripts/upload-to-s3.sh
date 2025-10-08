#!/bin/bash

# Real S3 management script for Terraform Registry
set -e

BUCKET="test-roomba-my-terraform-registry"
REGION="${AWS_REGION:-eu-west-1}"
S3_PREFIX="registry"
PROVIDERS_DIR="${PROVIDERS_DIR:-providers}"

# Functions
show_help() {
    echo "Real S3 Management Script"
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  list                    - List all objects in the bucket"
    echo "  upload [directory]      - Upload provider data to S3 (default: providers/)"
    echo "  download <key> [output] - Download a specific file"
    echo "  sync <directory>        - Sync local directory to S3 (careful!)"
    echo "  clean                   - Clean all data from bucket (dangerous!)"
    echo "  status                  - Show bucket status"
    echo "  help                    - Show this help"
    echo ""
    echo "Environment Variables Required:"
    echo "  AWS_ACCESS_KEY_ID     - AWS access key"
    echo "  AWS_SECRET_ACCESS_KEY - AWS secret key"
    echo ""
    echo "Environment Variables Optional:"
    echo "  AWS_REGION            - AWS region (default: eu-west-1)"
    echo "  PROVIDERS_DIR         - Providers directory (default: providers)"
    echo ""
    echo "Examples:"
    echo "  # Upload from default 'providers' directory"
    echo "  $0 upload"
    echo ""
    echo "  # Upload from custom directory"
    echo "  $0 upload ./my-providers"
    echo ""
    echo "  # Use environment variable"
    echo "  PROVIDERS_DIR=./custom-providers $0 upload"
}

check_credentials() {
    local missing=0
    
    if [ -z "$AWS_ACCESS_KEY_ID" ]; then
        echo "❌ AWS_ACCESS_KEY_ID environment variable is not set"
        missing=1
    fi
    
    if [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
        echo "❌ AWS_SECRET_ACCESS_KEY environment variable is not set"
        missing=1
    fi
    
    if [ $missing -eq 1 ]; then
        echo ""
        echo "Please set the required environment variables:"
        echo "  export AWS_ACCESS_KEY_ID=your-key"
        echo "  export AWS_SECRET_ACCESS_KEY=your-secret"
        echo "  export AWS_REGION=us-east-1  # optional, defaults to us-east-1"
        exit 1
    fi
    
    echo "✅ AWS credentials found"
}

check_bucket_exists() {
    echo "🔍 Checking if bucket '$BUCKET' exists..."
    if aws s3 ls "s3://$BUCKET" --region "$REGION" > /dev/null 2>&1; then
        echo "✅ Bucket '$BUCKET' exists and is accessible"
        return 0
    else
        echo "❌ Bucket '$BUCKET' does not exist or is not accessible"
        echo "   Please create the bucket first or check your permissions"
        return 1
    fi
}

list_objects() {
    echo "📋 Listing objects in bucket '$BUCKET/$S3_PREFIX':"
    aws s3 ls "s3://$BUCKET/$S3_PREFIX/" --recursive --human-readable --region "$REGION" || {
        echo "❌ Failed to list objects"
        echo "   Make sure you have s3:ListBucket permission"
        exit 1
    }
}

upload_providers() {
    local providers_dir="${1:-$PROVIDERS_DIR}"
    
    echo "📤 Uploading provider data to S3..."
    echo "   Source directory: $providers_dir"
    
    # Check if providers directory exists
    if [ ! -d "$providers_dir" ]; then
        echo "❌ Directory '$providers_dir' not found"
        echo "   Run the download-real-providers.sh script first to download providers"
        echo "   Or specify a different directory: $0 upload <directory>"
        exit 1
    fi
    
    # Count files to upload
    file_count=$(find "$providers_dir" -type f | wc -l)
    echo "📊 Found $file_count files to upload"
    
    if [ "$file_count" -eq 0 ]; then
        echo "❌ No files found in $providers_dir directory"
        exit 1
    fi
    
    # Ask for confirmation
    read -p "Upload $file_count files to s3://$BUCKET/$S3_PREFIX/providers/? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ Upload cancelled"
        exit 0
    fi
    
    # Upload with progress
    echo "⏳ Uploading..."
    aws s3 sync "$providers_dir" "s3://$BUCKET/$S3_PREFIX/providers/" \
        --region "$REGION" \
        --no-progress \
        || {
        echo "❌ Upload failed"
        exit 1
    }
    
    echo "✅ Upload completed successfully"
    echo ""
    echo "📊 Verifying upload..."
    uploaded_count=$(aws s3 ls "s3://$BUCKET/$S3_PREFIX/providers/" --recursive --region "$REGION" | wc -l)
    echo "✅ Total objects in S3: $uploaded_count"
}

sync_directory() {
    local local_dir="$1"
    
    if [ -z "$local_dir" ]; then
        echo "Usage: $0 sync <local-directory>"
        echo "Example: $0 sync ./providers"
        exit 1
    fi
    
    if [ ! -d "$local_dir" ]; then
        echo "❌ Directory '$local_dir' not found"
        exit 1
    fi
    
    # Ask for confirmation (sync deletes files not in source!)
    echo "⚠️  WARNING: This will sync '$local_dir' to s3://$BUCKET/$S3_PREFIX/"
    echo "    Files in S3 not present locally will be DELETED"
    read -p "Are you sure? (yes/N) " -r
    echo
    if [[ ! $REPLY == "yes" ]]; then
        echo "❌ Sync cancelled"
        exit 0
    fi
    
    echo "⏳ Syncing..."
    aws s3 sync "$local_dir" "s3://$BUCKET/$S3_PREFIX/" \
        --region "$REGION" \
        --delete \
        || {
        echo "❌ Sync failed"
        exit 1
    }
    
    echo "✅ Sync completed"
}

download_file() {
    local key="$1"
    local output_file="$2"
    
    if [ -z "$key" ]; then
        echo "Usage: $0 download <s3-key> [output-file]"
        echo "Example: $0 download providers/hashicorp/aws/5.20.1/metadata.json"
        exit 1
    fi
    
    # If no output file specified, use the basename of the key
    if [ -z "$output_file" ]; then
        output_file=$(basename "$key")
    fi
    
    echo "📥 Downloading: s3://$BUCKET/$S3_PREFIX/$key"
    aws s3 cp "s3://$BUCKET/$S3_PREFIX/$key" "$output_file" --region "$REGION" || {
        echo "❌ Failed to download file"
        exit 1
    }
    echo "✅ Downloaded to: $output_file"
}

clean_bucket() {
    echo "⚠️  WARNING: This will delete ALL objects in s3://$BUCKET/$S3_PREFIX/"
    echo "   This action CANNOT be undone!"
    echo ""
    read -p "Type 'DELETE ALL' to confirm: " -r
    echo
    if [[ ! $REPLY == "DELETE ALL" ]]; then
        echo "❌ Clean cancelled (confirmation didn't match)"
        exit 0
    fi
    
    echo "🧹 Cleaning bucket '$BUCKET/$S3_PREFIX/'..."
    aws s3 rm "s3://$BUCKET/$S3_PREFIX/" --recursive --region "$REGION" || {
        echo "❌ Failed to clean bucket"
        exit 1
    }
    echo "✅ Bucket cleaned"
}

show_status() {
    echo "🔍 S3 Bucket Status:"
    echo "   Bucket: $BUCKET"
    echo "   Region: $REGION"
    echo "   Prefix: $S3_PREFIX"
    echo ""
    
    if check_bucket_exists; then
        echo ""
        echo "📊 Object counts by provider:"
        
        # List providers and count objects
        aws s3 ls "s3://$BUCKET/$S3_PREFIX/providers/" --region "$REGION" | grep PRE | awk '{print $2}' | while read -r namespace; do
            if [ ! -z "$namespace" ]; then
                count=$(aws s3 ls "s3://$BUCKET/$S3_PREFIX/providers/${namespace}" --recursive --region "$REGION" | wc -l)
                printf "   %-30s %s objects\n" "${namespace}" "$count"
            fi
        done
        
        echo ""
        total_count=$(aws s3 ls "s3://$BUCKET/$S3_PREFIX/" --recursive --region "$REGION" | wc -l)
        echo "📈 Total objects: $total_count"
        
        # Estimate total size
        echo ""
        echo "💾 Storage usage:"
        aws s3 ls "s3://$BUCKET/$S3_PREFIX/" --recursive --human-readable --summarize --region "$REGION" | tail -n 2
    fi
}

# Main script
if [ $# -eq 0 ]; then
    show_help
    exit 0
fi

case "$1" in
    list)
        check_credentials
        check_bucket_exists || exit 1
        list_objects
        ;;
    upload)
        check_credentials
        check_bucket_exists || exit 1
        upload_providers "$2"
        ;;
    sync)
        check_credentials
        check_bucket_exists || exit 1
        sync_directory "$2"
        ;;
    download)
        check_credentials
        check_bucket_exists || exit 1
        download_file "$2" "$3"
        ;;
    clean)
        check_credentials
        check_bucket_exists || exit 1
        clean_bucket
        ;;
    status)
        check_credentials
        check_bucket_exists || exit 1
        show_status
        ;;
    help|*)
        show_help
        ;;
esac
