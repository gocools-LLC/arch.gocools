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

func TestFromResourcesInfersTopologyEdges(t *testing.T) {
	graph := FromResources([]model.Resource{
		{
			ID:       "i-1",
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
				"gocools:owner":       "platform-team",
			},
			Metadata: map[string]string{
				"vpc_id":    "vpc-123",
				"subnet_id": "subnet-123",
			},
		},
		{
			ID:       "lb-1",
			Type:     "aws.elbv2.load_balancer",
			Provider: "aws",
			Region:   "us-east-1",
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
				"gocools:owner":       "platform-team",
			},
			Metadata: map[string]string{
				"vpc_id": "vpc-123",
			},
		},
	}, time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC))

	vpcNodeID := "synthetic:vpc:us-east-1:vpc-123"
	subnetNodeID := "synthetic:subnet:us-east-1:subnet-123"

	if !hasNode(graph.Nodes, vpcNodeID) {
		t.Fatalf("expected synthetic vpc node %q", vpcNodeID)
	}
	if !hasNode(graph.Nodes, subnetNodeID) {
		t.Fatalf("expected synthetic subnet node %q", subnetNodeID)
	}
	if !hasEdge(graph.Edges, "i-1", subnetNodeID, "in_subnet") {
		t.Fatal("expected edge i-1 -> subnet (in_subnet)")
	}
	if !hasEdge(graph.Edges, subnetNodeID, vpcNodeID, "part_of") {
		t.Fatal("expected edge subnet -> vpc (part_of)")
	}
	if !hasEdge(graph.Edges, "lb-1", vpcNodeID, "in_vpc") {
		t.Fatal("expected edge lb-1 -> vpc (in_vpc)")
	}

	filtered := graph.Filter(Query{
		StackID:     "dev-stack",
		Environment: "dev",
	})
	if !hasNode(filtered.Nodes, vpcNodeID) || !hasNode(filtered.Nodes, subnetNodeID) {
		t.Fatal("expected filtered graph to retain inferred topology nodes")
	}
}

func hasNode(nodes []Node, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func hasEdge(edges []Edge, from string, to string, edgeType string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Type == edgeType {
			return true
		}
	}
	return false
}
