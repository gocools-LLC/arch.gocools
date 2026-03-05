package graph

import (
	"context"
	"testing"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

func TestServiceQueryAppliesFilters(t *testing.T) {
	service := NewService(NewStaticResourceProvider([]model.Resource{
		{
			ID:       "resource-dev",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
			},
		},
		{
			ID:       "resource-prod",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Tags: map[string]string{
				"gocools:stack-id":    "prod-stack",
				"gocools:environment": "prod",
			},
		},
	}))
	service.now = func() time.Time { return time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC) }

	result, err := service.Query(context.Background(), Query{
		StackID:     "prod-stack",
		Environment: "prod",
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(result.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result.Nodes))
	}
	if result.Nodes[0].ID != "resource-prod" {
		t.Fatalf("expected resource-prod, got %q", result.Nodes[0].ID)
	}
}
