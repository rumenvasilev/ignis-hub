# GitHub Actions Workflows

This directory contains CI/CD workflows for the Ignis Hub Terraform Registry.

## Workflows

### 🧪 [test.yml](test.yml)
**Unit Tests, Linting, and Security**

Runs on: Push to non-main branches, PRs to main

- **Unit Tests**: Go test suite with race detection and coverage
- **Linting**: golangci-lint for code quality
- **Security**: Gosec security scanner with SARIF upload
- **Coverage**: Uploads to Codecov

### 🚀 [e2e-test.yml](e2e-test.yml)
**End-to-End Integration Tests**

Runs on: Push to non-main branches, PRs to main, manual trigger

- **Full Stack**: LocalStack (S3) + Registry service
- **API Testing**: Comprehensive endpoint validation
- **Upload Testing**: Provider/module upload with ignishubctl (curl fallback)
- **Terraform Integration**: Real Terraform client testing
- **Artifacts**: Logs uploaded on failure

**Key Steps**:
1. Start LocalStack (S3 emulation)
2. Initialize S3 bucket and sync test fixtures from `testfixtures/`
3. Build registry binary (`ignis-hub`)
4. Build CLI tool (`ignishubctl`)
5. Start Terraform Registry
6. Run E2E test suite (`scripts/e2e-test.sh`) - includes:
   - API endpoint validation
   - Module/provider upload with ignishubctl
   - Module directory upload (--dir flag)
   - Provider cloning from official registry
   - Verification via ignishubctl list commands
7. Test Terraform client integration (`scripts/e2e-terraform-integration.sh`)
8. Cleanup

### 🏗️ [build.yml](build.yml)
**Build and Release Artifacts**

Runs on: Push to main, tags

- **Multi-platform**: Linux, macOS, Windows
- **Architectures**: amd64, arm64
- **Artifacts**: Binaries for all platforms
- **Docker**: Container image build and push

### 📊 [coverage.yml](coverage.yml)
**Code Coverage Reporting**

Runs on: Push to main

- **Coverage**: Generates and uploads coverage reports
- **Badges**: Updates coverage badges
- **Trends**: Tracks coverage over time

### 🎉 [release.yml](release.yml)
**GitHub Releases**

Runs on: Version tags (v*)

- **Binaries**: Builds for all platforms
- **Checksums**: SHA256 for all artifacts
- **Release Notes**: Auto-generated from commits
- **Assets**: Uploads all binaries to GitHub Release

## Workflow Dependencies

```
test.yml ──┐
           ├──> (Required for merge)
e2e-test.yml ┘

build.yml ──> (On main/tags)
coverage.yml ─> (On main)
release.yml ──> (On version tags)
```

## Local Testing

### Run E2E Tests Locally

```bash
# Start services (LocalStack + Registry)
docker-compose up -d

# Run E2E tests (builds ignishubctl automatically if needed)
./scripts/e2e-test.sh

# Cleanup
docker-compose down -v
```

### Validate Workflow Files

```bash
# Using actionlint
actionlint .github/workflows/*.yml

# Using GitHub CLI
gh workflow list
gh workflow run e2e-test.yml
```

## Secrets Required

### For Release Workflow
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions

### For Docker Push (if enabled)
- `DOCKERHUB_USERNAME` - Docker Hub username
- `DOCKERHUB_TOKEN` - Docker Hub access token

### For Coverage
- `CODECOV_TOKEN` - Codecov upload token

## Debugging Workflows

### View Logs

```bash
# List workflow runs
gh run list --workflow=e2e-test.yml

# View specific run
gh run view <run-id>

# Download logs
gh run download <run-id>
```

### Re-run Failed Jobs

```bash
# Re-run failed jobs
gh run rerun <run-id> --failed

# Re-run entire workflow
gh run rerun <run-id>
```

### Manual Trigger

```bash
# Trigger E2E tests manually
gh workflow run e2e-test.yml

# With specific branch
gh workflow run e2e-test.yml --ref my-branch
```

## Best Practices

1. **Fast Feedback**: Unit tests run first (fastest)
2. **Parallel Execution**: Independent jobs run in parallel
3. **Caching**: Go modules cached for faster builds
4. **Artifacts**: Logs saved on failure for debugging
5. **Security**: SARIF results uploaded to GitHub Security tab
6. **Coverage**: Tracked over time with Codecov

## Adding New Workflows

1. Create new `.yml` file in `.github/workflows/`
2. Define trigger events (`on:`)
3. Add jobs with steps
4. Test locally with `act` or push to branch
5. Update this README

Example:
```yaml
name: My New Workflow

on:
  push:
    branches: [ main ]

jobs:
  my-job:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v5
    - name: My Step
      run: echo "Hello World"
```

## Related Documentation

- [E2E Testing Guide](../../E2E-TESTING.md)
- [Local Development](../../README-local-development.md)
- [GitHub Actions Docs](https://docs.github.com/en/actions)

