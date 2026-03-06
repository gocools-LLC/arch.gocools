package main

import (
	"context"
	"testing"

	internalaws "github.com/gocools-LLC/arch.gocools/internal/aws"
)

func TestBuildGraphServiceStaticMode(t *testing.T) {
	service, err := buildGraphService(context.Background(), internalaws.RuntimeConfig{
		DiscoveryMode: internalaws.DiscoveryModeStatic,
	})
	if err != nil {
		t.Fatalf("buildGraphService returned error: %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil graph service for static mode, got %#v", service)
	}
}

func TestBuildGraphServiceUnknownMode(t *testing.T) {
	_, err := buildGraphService(context.Background(), internalaws.RuntimeConfig{
		DiscoveryMode: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
}

func TestBuildGraphServiceAWSModeRequiresRegion(t *testing.T) {
	_, err := buildGraphService(context.Background(), internalaws.RuntimeConfig{
		DiscoveryMode: internalaws.DiscoveryModeAWS,
	})
	if err == nil {
		t.Fatal("expected error when aws mode has no region")
	}
}
