package aws

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	DiscoveryModeStatic = "static"
	DiscoveryModeAWS    = "aws"
)

type RuntimeConfig struct {
	DiscoveryMode   string
	Session         SessionConfig
	ValidateOnStart bool
}

func RuntimeConfigFromEnv() RuntimeConfig {
	region := envOrDefault("ARCH_AWS_REGION", os.Getenv("AWS_REGION"))

	return RuntimeConfig{
		DiscoveryMode: envOrDefault("ARCH_DISCOVERY_MODE", DiscoveryModeStatic),
		Session: SessionConfig{
			Region:      region,
			RoleARN:     os.Getenv("ARCH_AWS_ROLE_ARN"),
			SessionName: envOrDefault("ARCH_AWS_SESSION_NAME", "arch-session"),
			ExternalID:  os.Getenv("ARCH_AWS_EXTERNAL_ID"),
		},
		ValidateOnStart: envBoolOrDefault("ARCH_AWS_VALIDATE_ON_START", false),
	}
}

func (c RuntimeConfig) NormalizedDiscoveryMode() string {
	mode := strings.ToLower(strings.TrimSpace(c.DiscoveryMode))
	if mode == "" {
		return DiscoveryModeStatic
	}
	return mode
}

func ValidateCredentials(ctx context.Context, session SessionConfig, optFns ...func(*config.LoadOptions) error) error {
	awsCfg, err := LoadConfig(ctx, session, optFns...)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	if awsCfg.Credentials == nil {
		return fmt.Errorf("credentials provider is not configured")
	}

	if _, err := awsCfg.Credentials.Retrieve(ctx); err != nil {
		return fmt.Errorf("retrieve aws credentials: %w", err)
	}
	return nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
