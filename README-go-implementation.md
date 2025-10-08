# Terraform Registry API - Go Implementation

This is a complete Go implementation of a private Terraform Registry API that serves modules and providers from AWS S3, built from the OpenAPI specification.

## Features

- **Full Terraform Registry Protocol Support**: Implements all endpoints for modules and providers
- **AWS S3 Backend**: Stores and serves binaries and metadata from S3
- **Multiple Authentication Methods**: Support for token, basic auth, and AWS IAM
- **Private Registry**: Designed for internal use with authentication and IP whitelisting
- **Structured Logging**: JSON/text logging with configurable levels
- **Health Checks**: Built-in health monitoring endpoints
- **Docker Support**: Complete containerization with docker-compose
- **Graceful Shutdown**: Proper signal handling and cleanup
- **CORS Support**: Ready for browser-based clients

## Project Structure

```
├── main.go                     # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   │   └── config.go
│   ├── models/                 # Data structures and API models
│   │   └── models.go
│   ├── storage/                # S3 storage implementation
│   │   └── s3.go
│   ├── handlers/               # HTTP request handlers
│   │   └── handlers.go
│   └── middleware/             # HTTP middleware
│       ├── auth.go             # Authentication middleware
│       └── logging.go          # Logging and CORS middleware
├── config.yaml                 # Configuration file
├── environment.example         # Environment variables template
├── Dockerfile                  # Container build configuration
├── docker-compose.yml          # Local development setup
└── go.mod                      # Go module dependencies
```

## API Endpoints

The implementation provides the following endpoints according to the Terraform Registry Protocol:

- `GET /.well-known/terraform.json` - Service discovery
- `GET /v1/modules/{namespace}/{name}/{system}/versions` - List module versions
- `GET /v1/modules/{namespace}/{name}/{system}/{version}/download` - Download module
- `GET /v1/providers/{namespace}/{type}/versions` - List provider versions
- `GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}` - Download provider
- `GET /health` - Health check endpoint
- `GET /` - Service information

## Configuration

The application can be configured using:

1. **YAML Configuration File**: `config.yaml`
2. **Environment Variables**: Prefixed with `REGISTRY_`
3. **Command Line Flags**: (can be extended)

### Configuration Options

#### Server Configuration
```yaml
server:
  port: 8080
  host: "0.0.0.0"
  base_url: "http://localhost:8080"
```

#### AWS S3 Configuration
```yaml
aws:
  region: "us-east-1"
  s3_bucket: "my-terraform-registry"
  s3_prefix: "registry"
  # Optional for development
  endpoint: "http://localhost:4566"  # LocalStack
  access_key_id: "your-key"
  secret_access_key: "your-secret"
```

#### Authentication Configuration
```yaml
auth:
  enabled: true
  method: "token"  # Options: "basic", "token", "iam"
  token: "your-secret-token"
  # For basic auth:
  # username: "admin"
  # password: "password"
  # IP whitelist:
  # allowed_ips:
  #   - "192.168.1.0/24"
  #   - "10.0.0.0/8"
```

#### Logging Configuration
```yaml
log:
  level: "info"  # debug, info, warn, error
  format: "json"  # json, text
```

## S3 Storage Structure

The implementation expects the following S3 structure:

```
s3://{bucket}/{prefix}/
├── modules/
│   └── {namespace}/
│       └── {name}/
│           └── {system}/
│               ├── metadata.json
│               └── {version}/
│                   └── archive.tar.gz
└── providers/
    └── {namespace}/
        └── {type}/
            ├── metadata.json
            └── {version}/
                └── {os}/
                    └── {arch}/
                        ├── metadata.json
                        └── terraform-provider-{type}_{version}_{os}_{arch}.zip
```

### Metadata File Examples

#### Module Metadata (`metadata.json`)
```json
{
  "namespace": "terraform-aws-modules",
  "name": "vpc",
  "system": "aws",
  "versions": [
    {"version": "3.0.0"},
    {"version": "2.9.0"}
  ]
}
```

#### Provider Metadata (`metadata.json`)
```json
{
  "namespace": "hashicorp",
  "type": "aws",
  "versions": [
    {
      "version": "5.20.1",
      "protocols": ["5.0"],
      "platforms": [
        {"os": "darwin", "arch": "amd64"},
        {"os": "linux", "arch": "amd64"}
      ]
    }
  ]
}
```

#### Provider Binary Metadata
```json
{
  "namespace": "hashicorp",
  "type": "aws",
  "version": "5.20.1",
  "os": "darwin",
  "arch": "amd64",
  "filename": "terraform-provider-aws_5.20.1_darwin_amd64.zip",
  "download_url": "https://s3.amazonaws.com/...",
  "shasum": "abc123...",
  "shasums_url": "https://s3.amazonaws.com/.../shasums",
  "shasums_signature_url": "https://s3.amazonaws.com/.../shasums.sig",
  "protocols": ["5.0"],
  "signing_keys": {
    "gpg_public_keys": []
  }
}
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for local development)
- AWS S3 bucket (or LocalStack for development)

### Local Development

1. **Clone and setup**:
   ```bash
   git clone <repository>
   cd terraform-registry-api
   go mod tidy
   ```

2. **Start with Docker Compose**:
   ```bash
   docker-compose up -d
   ```
   This starts the registry with LocalStack for S3 simulation.

3. **Configure environment**:
   ```bash
   cp environment.example .env
   # Edit .env with your settings
   ```

4. **Run locally**:
   ```bash
   go run main.go
   ```

### Production Deployment

1. **Build the binary**:
   ```bash
   go build -o terraform-registry main.go
   ```

2. **Create configuration**:
   ```bash
   cp config.yaml /etc/terraform-registry/config.yaml
   # Edit configuration for production
   ```

3. **Run with systemd** (example service file):
   ```ini
   [Unit]
   Description=Terraform Registry API
   After=network.target

   [Service]
   Type=simple
   User=terraform-registry
   WorkingDirectory=/opt/terraform-registry
   ExecStart=/opt/terraform-registry/terraform-registry
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```

### Docker Deployment

1. **Build container**:
   ```bash
   docker build -t terraform-registry .
   ```

2. **Run container**:
   ```bash
   docker run -d \
     -p 8080:8080 \
     -e REGISTRY_AWS_S3_BUCKET=my-bucket \
     -e REGISTRY_AUTH_TOKEN=secret-token \
     terraform-registry
   ```

## Authentication

### Token Authentication
```bash
curl -H "Authorization: Bearer your-token" \
  http://localhost:8080/.well-known/terraform.json
```

### Basic Authentication
```bash
curl -u username:password \
  http://localhost:8080/.well-known/terraform.json
```

### Using with Terraform

Configure Terraform to use your private registry:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "your-registry.com/hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
```

Configure authentication in `~/.terraformrc`:
```hcl
credentials "your-registry.com" {
  token = "your-secret-token"
}
```

## Monitoring and Operations

### Health Checks
```bash
curl http://localhost:8080/health
```

### Logs
The application provides structured logging in JSON or text format. Key log fields include:
- Request ID correlation
- HTTP status codes and latency
- Authentication events
- S3 operation results
- Error tracking

### Metrics
The application is ready for metrics integration. You can extend it with:
- Prometheus metrics
- AWS CloudWatch
- Custom monitoring solutions

## Security Considerations

1. **Use HTTPS in production** - Configure TLS termination
2. **Secure authentication tokens** - Use strong, unique tokens
3. **Network security** - Use VPC endpoints for S3 access
4. **IAM permissions** - Follow least privilege principle
5. **Logging** - Monitor authentication failures and access patterns

## Extending the Implementation

### Adding New Storage Backends
Implement the storage interface to support other backends:
```go
type StorageInterface interface {
    GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error)
    GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error)
    // ... other methods
}
```

### Custom Authentication
Extend the authentication middleware in `internal/middleware/auth.go` to support:
- OIDC/OAuth2
- Custom API key validation
- Integration with existing auth systems

### Monitoring Integration
Add middleware for:
- Prometheus metrics
- Distributed tracing
- Custom monitoring systems

## Troubleshooting

### Common Issues

1. **S3 Access Denied**:
   - Check AWS credentials and IAM permissions
   - Verify bucket name and region
   - For LocalStack, ensure endpoint is correct

2. **Authentication Failures**:
   - Verify token/credentials configuration
   - Check IP whitelist settings
   - Review authentication method settings

3. **Module/Provider Not Found**:
   - Verify S3 key structure matches expected format
   - Check metadata.json files are valid JSON
   - Ensure proper S3 prefix configuration

4. **Health Check Failures**:
   - Check S3 connectivity
   - Verify AWS credentials
   - Review network connectivity

### Debug Mode
Enable debug logging:
```yaml
log:
  level: "debug"
```
Or via environment:
```bash
export REGISTRY_LOG_LEVEL=debug
```

## Contributing

This implementation provides a solid foundation for a private Terraform registry. Areas for contribution include:

1. Enhanced authentication methods
2. Additional storage backends
3. Metrics and monitoring integration
4. Performance optimizations
5. Additional security features

## License

This implementation follows the same license as the original Smithy API specification. 
