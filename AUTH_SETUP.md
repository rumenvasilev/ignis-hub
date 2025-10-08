# 🔐 Registry Authentication Setup

This guide shows how to enable and configure authentication for your private Terraform registry.

## 🛡️ Enable Authentication on Registry

### 1. **Token Authentication** (Recommended)

```bash
# Enable token authentication
export REGISTRY_AUTH_ENABLED=true
export REGISTRY_AUTH_METHOD=token
export REGISTRY_AUTH_TOKEN=my-secret-registry-token-2025

# Start registry
go run main.go
```

### 2. **Basic Authentication**

```bash
# Enable basic authentication  
export REGISTRY_AUTH_ENABLED=true
export REGISTRY_AUTH_METHOD=basic
export REGISTRY_AUTH_USERNAME=admin
export REGISTRY_AUTH_PASSWORD=secure-password-123

# Start registry
go run main.go
```

### 3. **IP Whitelisting** (Optional)

```bash
# Add IP restrictions
export REGISTRY_AUTH_ALLOWED_IPS="192.168.1.0/24,10.0.0.0/8,127.0.0.1"
```

## 🏗️ Configure Terraform Client

### Method 1: Token Authentication (.terraformrc)

Create or update `~/.terraformrc`:

```hcl
# Host configuration
host "localhost:8080" {
  services = {
    "providers.v1" = "http://localhost:8080/v1/providers/",
    "modules.v1" = "http://localhost:8080/v1/modules/",
  }
  insecure = true
}

# Token authentication
credentials "localhost:8080" {
  token = "my-secret-registry-token-2025"
}
```

### Method 2: Environment Variable

```bash
# Set token via environment variable
export TF_TOKEN_localhost_8080="my-secret-registry-token-2025"

# Run terraform
terraform init
```

### Method 3: Basic Authentication (Not recommended)

Terraform doesn't directly support basic auth in credentials block, but you can use:

```bash
# Manual curl test with basic auth
curl -u admin:secure-password-123 \
  http://localhost:8080/.well-known/terraform.json
```

## 🧪 Test Authentication

### 1. **Test Registry Endpoints**

```bash
# Without authentication (should fail)
curl http://localhost:8080/.well-known/terraform.json

# With token authentication
curl -H "Authorization: Bearer my-secret-registry-token-2025" \
  http://localhost:8080/.well-known/terraform.json

# With basic authentication  
curl -u admin:secure-password-123 \
  http://localhost:8080/.well-known/terraform.json
```

### 2. **Test Terraform Init**

```bash
cd examples/terraform

# This should work if credentials are configured
terraform init

# This should show provider download from your registry
terraform init -upgrade
```

## 🔧 HTTP Headers Used

The auth middleware accepts these authentication methods:

| Method | Header | Example |
|--------|--------|---------|
| **Token** | `Authorization: Bearer <token>` | `Authorization: Bearer my-secret-token` |
| **Token** | `X-API-Token: <token>` | `X-API-Token: my-secret-token` |
| **Basic** | `Authorization: Basic <base64>` | `Authorization: Basic YWRtaW46cGFzcw==` |
| **Query** | `?token=<token>` | `http://localhost:8080/health?token=my-token` |

## 🚨 Security Notes

1. **Use HTTPS in production** - HTTP is only for local development
2. **Strong tokens** - Use cryptographically secure random tokens
3. **IP whitelisting** - Restrict access to known networks
4. **Token rotation** - Regularly rotate authentication tokens
5. **Environment variables** - Never commit tokens to version control

## 📋 Example Production Config

```yaml
# config.yaml for production
auth:
  enabled: true
  method: "token"
  token: "${REGISTRY_TOKEN}"  # Read from environment
  allowed_ips:
    - "10.0.0.0/8"      # Internal network
    - "192.168.1.0/24"   # Office network

server:
  port: 443
  host: "0.0.0.0"
  base_url: "https://terraform-registry.mycompany.com"

log:
  level: "info"
  format: "json"
```

## 🎯 Quick Start with Auth

```bash
# 1. Enable token auth
export REGISTRY_AUTH_ENABLED=true
export REGISTRY_AUTH_METHOD=token
export REGISTRY_AUTH_TOKEN=test-token-123

# 2. Start registry
go run main.go

# 3. Configure Terraform
echo 'credentials "localhost:8080" { token = "test-token-123" }' >> ~/.terraformrc

# 4. Test
cd examples/terraform && terraform init
```
