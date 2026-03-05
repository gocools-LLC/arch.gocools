package graph

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
)

func TestFromResourcesBuildsVersionedGraph(t *testing.T) {
	graph := FromResources([]model.Resource{
		{
			ID:       "i-1",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
			},
		},
	}, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))

	if graph.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %q, got %q", SchemaVersion, graph.SchemaVersion)
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}
	if graph.Nodes[0].ID != "i-1" {
		t.Fatalf("expected node id i-1, got %q", graph.Nodes[0].ID)
	}
}

func TestGraphFilterByStackAndEnvironment(t *testing.T) {
	base := Graph{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		Nodes: []Node{
			{
				ID: "resource-1",
				Tags: map[string]string{
					"gocools:stack-id":    "dev-stack",
					"gocools:environment": "dev",
				},
			},
			{
				ID: "resource-2",
				Tags: map[string]string{
					"gocools:stack-id":    "prod-stack",
					"gocools:environment": "prod",
				},
			},
		},
	}

	filtered := base.Filter(Query{
		StackID:     "dev-stack",
		Environment: "dev",
	})

	if len(filtered.Nodes) != 1 {
		t.Fatalf("expected 1 filtered node, got %d", len(filtered.Nodes))
	}
	if filtered.Nodes[0].ID != "resource-1" {
		t.Fatalf("expected filtered node resource-1, got %q", filtered.Nodes[0].ID)
	}
}

func TestSchemaCompatibilityV1(t *testing.T) {
	const previousPayload = `{
	  "schema_version":"arch.gocools/v1alpha1",
	  "generated_at":"2026-03-05T00:00:00Z",
	  "nodes":[
	    {
	      "id":"resource-1",
	      "type":"aws.ec2.instance",
	      "provider":"aws",
	      "region":"us-east-1",
	      "tags":{"gocools:stack-id":"dev-stack","gocools:environment":"dev"},
	      "metadata":{"instance_type":"t3.micro"}
	    }
	  ],
	  "edges":[]
	}`

	var graph Graph
	if err := json.Unmarshal([]byte(previousPayload), &graph); err != nil {
		t.Fatalf("failed to unmarshal previous schema payload: %v", err)
	}
	if graph.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %q, got %q", SchemaVersion, graph.SchemaVersion)
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}

	reencoded, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("failed to marshal graph: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(reencoded, &payload); err != nil {
		t.Fatalf("failed to unmarshal reencoded payload: %v", err)
	}
	if _, ok := payload["schema_version"]; !ok {
		t.Fatal("expected schema_version field in encoded payload")
	}
	if _, ok := payload["nodes"]; !ok {
		t.Fatal("expected nodes field in encoded payload")
	}
	if _, ok := payload["edges"]; !ok {
		t.Fatal("expected edges field in encoded payload")
	}
}
