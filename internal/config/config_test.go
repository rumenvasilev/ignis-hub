package config

import (
	"os"
	"testing"

	"github.com/matryer/is"
	"github.com/spf13/viper"
)

func TestValidateConfig_AWS(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid AWS config",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "my-bucket",
					Region:   "us-east-1",
				},
			},
			wantErr: false,
		},
		{
			name: "missing S3 bucket",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					Region: "us-east-1",
				},
			},
			wantErr: true,
			errMsg:  "aws.s3_bucket is required when using AWS provider",
		},
		{
			name: "missing region",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "my-bucket",
				},
			},
			wantErr: true,
			errMsg:  "aws.region is required when using AWS provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			err := validateConfig(tt.config)

			if tt.wantErr {
				is.True(err != nil)              // should return error
				is.Equal(err.Error(), tt.errMsg) // error message should match
			} else {
				is.NoErr(err) // should not return error
			}
		})
	}
}

func TestValidateConfig_GCP(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GCP config",
			config: &Config{
				Provider: "gcp",
				GCP: GCPConfig{
					GCSBucket: "my-bucket",
				},
			},
			wantErr: false,
		},
		{
			name: "missing GCS bucket",
			config: &Config{
				Provider: "gcp",
				GCP:      GCPConfig{},
			},
			wantErr: true,
			errMsg:  "gcp.gcs_bucket is required when using GCP provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			err := validateConfig(tt.config)

			if tt.wantErr {
				is.True(err != nil)              // should return error
				is.Equal(err.Error(), tt.errMsg) // error message should match
			} else {
				is.NoErr(err) // should not return error
			}
		})
	}
}

func TestValidateConfig_UnsupportedProvider(t *testing.T) {
	is := is.New(t)
	config := &Config{
		Provider: "azure",
	}

	err := validateConfig(config)
	is.True(err != nil)                                  // should return error for unsupported provider
	is.Equal(err.Error(), "unsupported provider: azure") // error message should match
}

func TestValidateConfig_Auth(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "auth disabled",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "valid basic auth",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled:  true,
					Method:   "basic",
					Username: "admin",
					Password: "secret",
				},
			},
			wantErr: false,
		},
		{
			name: "basic auth missing username",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled:  true,
					Method:   "basic",
					Password: "secret",
				},
			},
			wantErr: true,
			errMsg:  "auth.username and auth.password are required for basic auth",
		},
		{
			name: "valid token auth",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled: true,
					Method:  "token",
					Token:   "secret-token",
				},
			},
			wantErr: false,
		},
		{
			name: "token auth missing token",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled: true,
					Method:  "token",
				},
			},
			wantErr: true,
			errMsg:  "auth.token is required for token auth",
		},
		{
			name: "valid IAM auth",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled: true,
					Method:  "iam",
				},
			},
			wantErr: false,
		},
		{
			name: "unsupported auth method",
			config: &Config{
				Provider: "aws",
				AWS: AWSConfig{
					S3Bucket: "bucket",
					Region:   "us-east-1",
				},
				Auth: AuthConfig{
					Enabled: true,
					Method:  "oauth",
				},
			},
			wantErr: true,
			errMsg:  "unsupported auth method: oauth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			err := validateConfig(tt.config)

			if tt.wantErr {
				is.True(err != nil)              // should return error
				is.Equal(err.Error(), tt.errMsg) // error message should match
			} else {
				is.NoErr(err) // should not return error
			}
		})
	}
}

func TestLoad_FromEnv_AWS(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Set environment variables
	os.Setenv("REGISTRY_AWS_S3_BUCKET", "test-bucket")
	os.Setenv("REGISTRY_AWS_REGION", "eu-west-1")
	os.Setenv("REGISTRY_AUTH_ENABLED", "false")
	defer func() {
		os.Unsetenv("REGISTRY_AWS_S3_BUCKET")
		os.Unsetenv("REGISTRY_AWS_REGION")
		os.Unsetenv("REGISTRY_AUTH_ENABLED")
	}()

	cfg, err := Load("aws")
	is.NoErr(err) // Load should succeed

	is.Equal(cfg.AWS.S3Bucket, "test-bucket") // S3Bucket should match env var
	is.Equal(cfg.AWS.Region, "eu-west-1")     // Region should match env var
	is.Equal(cfg.Provider, "aws")             // Provider should be set to aws
}

func TestLoad_FromEnv_GCP(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Set environment variables
	os.Setenv("REGISTRY_GCS_BUCKET", "test-gcs-bucket")
	os.Setenv("REGISTRY_AUTH_ENABLED", "false")
	defer func() {
		os.Unsetenv("REGISTRY_GCS_BUCKET")
		os.Unsetenv("REGISTRY_AUTH_ENABLED")
	}()

	cfg, err := Load("gcp")
	is.NoErr(err) // Load should succeed

	is.Equal(cfg.GCP.GCSBucket, "test-gcs-bucket") // GCSBucket should match env var
	is.Equal(cfg.Provider, "gcp")                  // Provider should be set to gcp
}

func TestLoad_Defaults(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Set minimal required config
	os.Setenv("REGISTRY_AWS_S3_BUCKET", "bucket")
	os.Setenv("REGISTRY_AUTH_ENABLED", "false")
	defer func() {
		os.Unsetenv("REGISTRY_AWS_S3_BUCKET")
		os.Unsetenv("REGISTRY_AUTH_ENABLED")
	}()

	cfg, err := Load("aws")
	is.NoErr(err) // Load should succeed

	// Test defaults
	is.Equal(cfg.Server.Port, 8080)        // Server.Port should default to 8080
	is.Equal(cfg.Server.Host, "0.0.0.0")   // Server.Host should default to 0.0.0.0
	is.Equal(cfg.AWS.Region, "us-east-1")  // AWS.Region should default to us-east-1
	is.Equal(cfg.AWS.S3Prefix, "registry") // AWS.S3Prefix should default to registry
	is.Equal(cfg.Log.Level, "info")        // Log.Level should default to info
	is.Equal(cfg.Log.Format, "json")       // Log.Format should default to json
}

func TestLoad_MissingRequiredConfig(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Don't set required S3 bucket
	os.Setenv("REGISTRY_AUTH_ENABLED", "false")
	defer os.Unsetenv("REGISTRY_AUTH_ENABLED")

	_, err := Load("aws")
	is.True(err != nil) // Load should fail for missing S3 bucket
}

func TestLoad_AuthValidation(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Set required config but invalid auth
	os.Setenv("REGISTRY_AWS_S3_BUCKET", "bucket")
	os.Setenv("REGISTRY_AUTH_ENABLED", "true")
	os.Setenv("REGISTRY_AUTH_METHOD", "token")
	// Missing REGISTRY_AUTH_TOKEN
	defer func() {
		os.Unsetenv("REGISTRY_AWS_S3_BUCKET")
		os.Unsetenv("REGISTRY_AUTH_ENABLED")
		os.Unsetenv("REGISTRY_AUTH_METHOD")
	}()

	_, err := Load("aws")
	is.True(err != nil)                                                                      // Load should fail for missing auth token
	is.Equal(err.Error(), "config validation failed: auth.token is required for token auth") // error message should match
}

func TestLoad_EnvironmentBindings(t *testing.T) {
	is := is.New(t)
	// Reset viper before test
	viper.Reset()

	// Test that all environment variable bindings work
	envVars := map[string]string{
		"REGISTRY_AWS_ENDPOINT":          "http://localhost:4566",
		"REGISTRY_AWS_ACCESS_KEY_ID":     "test-key",
		"REGISTRY_AWS_SECRET_ACCESS_KEY": "test-secret",
		"REGISTRY_AWS_SESSION_TOKEN":     "test-token",
		"REGISTRY_AWS_S3_BUCKET":         "test-bucket",
		"REGISTRY_AWS_S3_PREFIX":         "test-prefix",
		"REGISTRY_AWS_REGION":            "us-west-2",
		"REGISTRY_GCS_BUCKET":            "gcs-bucket",
		"REGISTRY_GCS_PREFIX":            "gcs-prefix",
		"REGISTRY_GCP_ENDPOINT":          "http://localhost:8080",
		"REGISTRY_GCP_CREDENTIALS_FILE":  "/path/to/creds.json",
		"REGISTRY_AUTH_ENABLED":          "false",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, err := Load("aws")
	is.NoErr(err) // Load should succeed

	// Verify AWS bindings
	is.Equal(cfg.AWS.Endpoint, "http://localhost:4566") // AWS.Endpoint should match env var
	is.Equal(cfg.AWS.AccessKeyID, "test-key")           // AWS.AccessKeyID should match env var
	is.Equal(cfg.AWS.SecretAccessKey, "test-secret")    // AWS.SecretAccessKey should match env var

	// Verify GCP bindings
	is.Equal(cfg.GCP.GCSBucket, "gcs-bucket")                // GCP.GCSBucket should match env var
	is.Equal(cfg.GCP.CredentialsFile, "/path/to/creds.json") // GCP.CredentialsFile should match env var
}
