package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig `mapstructure:"server"`
	AWS      AWSConfig    `mapstructure:"aws"`
	GCP      GCPConfig    `mapstructure:"gcp"`
	Auth     AuthConfig   `mapstructure:"auth"`
	Log      LogConfig    `mapstructure:"log"`
	Provider string       // Active provider (aws or gcp)
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	Host    string `mapstructure:"host"`
	BaseURL string `mapstructure:"base_url"`
}

type AWSConfig struct {
	Region          string `mapstructure:"region"`
	S3Bucket        string `mapstructure:"s3_bucket"`
	S3Prefix        string `mapstructure:"s3_prefix"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
	Endpoint        string `mapstructure:"endpoint"` // For local development with LocalStack
}

type GCPConfig struct {
	GCSBucket       string `mapstructure:"gcs_bucket"`
	GCSPrefix       string `mapstructure:"gcs_prefix"`
	Endpoint        string `mapstructure:"endpoint"`         // For local development
	CredentialsFile string `mapstructure:"credentials_file"` // For local development
}

type AuthConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	Method     string   `mapstructure:"method"`      // "basic", "token", "iam"
	Username   string   `mapstructure:"username"`    // For basic auth
	Password   string   `mapstructure:"password"`    // For basic auth
	Token      string   `mapstructure:"token"`       // For token auth
	AllowedIPs []string `mapstructure:"allowed_ips"` // IP whitelist
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // "json" or "text"
}

func Load(provider string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/terraform-registry")

	// Set defaults
	setDefaults()

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("REGISTRY")

	// Explicitly bind nested AWS environment variables that Viper doesn't auto-map
	viper.BindEnv("aws.endpoint", "REGISTRY_AWS_ENDPOINT")                   //nolint:errcheck
	viper.BindEnv("aws.access_key_id", "REGISTRY_AWS_ACCESS_KEY_ID")         //nolint:errcheck
	viper.BindEnv("aws.secret_access_key", "REGISTRY_AWS_SECRET_ACCESS_KEY") //nolint:errcheck
	viper.BindEnv("aws.session_token", "REGISTRY_AWS_SESSION_TOKEN")         //nolint:errcheck
	viper.BindEnv("aws.s3_bucket", "REGISTRY_AWS_S3_BUCKET")                 //nolint:errcheck
	viper.BindEnv("aws.s3_prefix", "REGISTRY_AWS_S3_PREFIX")                 //nolint:errcheck
	viper.BindEnv("aws.region", "REGISTRY_AWS_REGION")                       //nolint:errcheck

	// Explicitly bind nested GCP environment variables that Viper doesn't auto-map
	viper.BindEnv("gcp.gcs_bucket", "REGISTRY_GCS_BUCKET")                 //nolint:errcheck
	viper.BindEnv("gcp.gcs_prefix", "REGISTRY_GCS_PREFIX")                 //nolint:errcheck
	viper.BindEnv("gcp.endpoint", "REGISTRY_GCP_ENDPOINT")                 //nolint:errcheck
	viper.BindEnv("gcp.credentials_file", "REGISTRY_GCP_CREDENTIALS_FILE") //nolint:errcheck

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, will use defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Set the active provider
	config.Provider = provider

	// Validate required fields for the active provider
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.base_url", "http://localhost:8080")

	// AWS defaults
	viper.SetDefault("aws.region", "us-east-1")
	viper.SetDefault("aws.s3_prefix", "registry")

	// GCP defaults
	viper.SetDefault("gcp.gcs_prefix", "registry")

	// Auth defaults
	viper.SetDefault("auth.enabled", true)
	viper.SetDefault("auth.method", "token")

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}

func validateConfig(config *Config) error {
	// Validate provider-specific configuration
	switch config.Provider {
	case "aws":
		if config.AWS.S3Bucket == "" {
			return fmt.Errorf("aws.s3_bucket is required when using AWS provider")
		}
		if config.AWS.Region == "" {
			return fmt.Errorf("aws.region is required when using AWS provider")
		}
	case "gcp":
		if config.GCP.GCSBucket == "" {
			return fmt.Errorf("gcp.gcs_bucket is required when using GCP provider")
		}
	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	// Validate auth configuration (common to all providers)
	if config.Auth.Enabled {
		switch config.Auth.Method {
		case "basic":
			if config.Auth.Username == "" || config.Auth.Password == "" {
				return fmt.Errorf("auth.username and auth.password are required for basic auth")
			}
		case "token":
			if config.Auth.Token == "" {
				return fmt.Errorf("auth.token is required for token auth")
			}
		case "iam":
			// IAM auth doesn't require additional config
		default:
			return fmt.Errorf("unsupported auth method: %s", config.Auth.Method)
		}
	}

	return nil
}
