# E2E Test Module (dir-upload)

A simple test module for e2e testing of the `--dir` flag in ignishubctl.

## Usage

```hcl
module "test" {
  source = "localhost:8080/e2e-test/dir-upload/aws"
  
  name        = "world"
  environment = "production"
}
```

## Inputs

| Name | Description | Type | Default |
|------|-------------|------|---------|
| name | Name for the resource | string | "test" |
| environment | Environment name | string | "dev" |

## Outputs

| Name | Description |
|------|-------------|
| greeting | A greeting message |
| timestamp | Current timestamp |

