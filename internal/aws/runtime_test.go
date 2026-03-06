package aws

import "testing"

func TestRuntimeConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("ARCH_DISCOVERY_MODE", "")
	t.Setenv("ARCH_AWS_REGION", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("ARCH_AWS_ROLE_ARN", "")
	t.Setenv("ARCH_AWS_SESSION_NAME", "")
	t.Setenv("ARCH_AWS_EXTERNAL_ID", "")
	t.Setenv("ARCH_AWS_VALIDATE_ON_START", "")

	cfg := RuntimeConfigFromEnv()

	if cfg.NormalizedDiscoveryMode() != DiscoveryModeStatic {
		t.Fatalf("expected default discovery mode %q, got %q", DiscoveryModeStatic, cfg.NormalizedDiscoveryMode())
	}
	if cfg.Session.Region != "" {
		t.Fatalf("expected empty default region, got %q", cfg.Session.Region)
	}
	if cfg.Session.SessionName != "arch-session" {
		t.Fatalf("expected default session name arch-session, got %q", cfg.Session.SessionName)
	}
	if cfg.ValidateOnStart {
		t.Fatal("expected validate_on_start default to false")
	}
}

func TestRuntimeConfigFromEnvUsesArchAndAWSRegionFallback(t *testing.T) {
	t.Setenv("ARCH_DISCOVERY_MODE", "AWS")
	t.Setenv("ARCH_AWS_REGION", "")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("ARCH_AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/arch-observer")
	t.Setenv("ARCH_AWS_SESSION_NAME", "custom-session")
	t.Setenv("ARCH_AWS_EXTERNAL_ID", "external-id")
	t.Setenv("ARCH_AWS_VALIDATE_ON_START", "true")

	cfg := RuntimeConfigFromEnv()

	if cfg.NormalizedDiscoveryMode() != DiscoveryModeAWS {
		t.Fatalf("expected normalized discovery mode %q, got %q", DiscoveryModeAWS, cfg.NormalizedDiscoveryMode())
	}
	if cfg.Session.Region != "us-west-2" {
		t.Fatalf("expected region fallback from AWS_REGION, got %q", cfg.Session.Region)
	}
	if cfg.Session.RoleARN != "arn:aws:iam::123456789012:role/arch-observer" {
		t.Fatalf("unexpected role arn: %q", cfg.Session.RoleARN)
	}
	if cfg.Session.SessionName != "custom-session" {
		t.Fatalf("unexpected session name: %q", cfg.Session.SessionName)
	}
	if cfg.Session.ExternalID != "external-id" {
		t.Fatalf("unexpected external id: %q", cfg.Session.ExternalID)
	}
	if !cfg.ValidateOnStart {
		t.Fatal("expected validate_on_start to be true")
	}
}
