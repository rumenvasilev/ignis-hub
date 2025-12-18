# terraform-registry

[![codecov](https://codecov.io/gh/rumenvasilev/ignis-hub/graph/badge.svg?token=J3XK07F806)](https://codecov.io/gh/rumenvasilev/ignis-hub)
[![E2E Tests](https://github.com/rumenvasilev/ignis-hub/actions/workflows/e2e-test.yml/badge.svg)](https://github.com/rumenvasilev/ignis-hub/actions/workflows/e2e-test.yml)

Terraform registry implementation, based off [terraform-registry-api](https://github.com/rumenvasilev/terraform-registry-api/)

## Documentation

- **[E2E Testing Guide](E2E-TESTING.md)** - Complete end-to-end testing documentation
- **[Local Development](README-local-development.md)** - Local development setup
- **[S3 Quick Start](QUICK-START-S3.md)** - S3 backend setup guide

## Quick Start

```bash
# Start the full stack (LocalStack + Registry)
docker-compose up -d

# Run E2E tests
./scripts/e2e-test.sh

# Test API endpoints
./scripts/test-api.sh
```

## Features

- ✅ Full Terraform Registry Protocol support
- ✅ S3 and GCS storage backends
- ✅ Provider and module hosting
- ✅ Authentication support (token-based)
- ✅ Docker Compose for local development
- ✅ Comprehensive E2E testing
- ✅ GitHub Actions CI/CD
