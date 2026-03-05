package tags

import (
	"strings"
	"testing"
)

func TestValidateRequiredTagsSuccess(t *testing.T) {
	err := Validate(map[string]string{
		"gocools:stack-id":    "dev-stack",
		"gocools:environment": "dev",
		"gocools:owner":       "alice",
	}, "dev-stack", "dev")
	if err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestValidateRequiredTagsFailureIncludesRemediation(t *testing.T) {
	err := Validate(map[string]string{
		"gocools:stack-id": "dev-stack",
	}, "dev-stack", "dev")
	if err == nil {
		t.Fatal("expected validation failure for missing required tags")
	}

	message := err.Error()
	if !strings.Contains(message, "missing required tags") || !strings.Contains(message, "remediation") {
		t.Fatalf("expected remediation hint in error, got %q", message)
	}
}

func TestValidateStackAndEnvironmentMismatch(t *testing.T) {
	err := Validate(map[string]string{
		"gocools:stack-id":    "wrong-stack",
		"gocools:environment": "dev",
		"gocools:owner":       "alice",
	}, "dev-stack", "dev")
	if err == nil {
		t.Fatal("expected stack-id mismatch validation error")
	}
}
