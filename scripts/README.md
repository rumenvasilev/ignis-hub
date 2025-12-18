# Scripts

| Script | Purpose |
|--------|---------|
| `e2e-test.sh` | API test suite |
| `e2e-upload-provider.sh` | Upload test provider to S3 |
| `e2e-verify-provider.sh` | Verify provider via API |
| `e2e-terraform-integration.sh` | Test Terraform client |
| `test-api.sh` | Quick API smoke tests |
| `manage-localstack.sh` | LocalStack management (start/stop/logs) |
| `upload-to-s3.sh` | Upload to real AWS S3 |
| `download-real-providers.sh` | Download providers from HashiCorp |
| `setup-terraform.sh` | Setup Terraform locally |
| `verify-setup.sh` | Verify local setup |

## Usage

```bash
# E2E testing (see E2E-TESTING.md for details)
./scripts/e2e-test.sh

# LocalStack
./scripts/manage-localstack.sh start|stop|logs

# Real S3 upload
export AWS_ACCESS_KEY_ID=xxx
export AWS_SECRET_ACCESS_KEY=xxx
./scripts/upload-to-s3.sh upload
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:8080` | Registry URL |
| `AWS_ENDPOINT` | `http://localhost:4566` | LocalStack endpoint |
| `S3_BUCKET` | `terraform-registry` | S3 bucket |

See [E2E-TESTING.md](../E2E-TESTING.md) for complete testing documentation.
