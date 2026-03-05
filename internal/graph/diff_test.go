package graph

import (
	"fmt"
	"reflect"
	"testing"
)

func TestDiffGraphsDeterministicOrderingAndCounts(t *testing.T) {
	before := Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{ID: "b", Type: "aws.ec2.instance", State: "running"},
			{ID: "a", Type: "aws.ec2.instance", State: "running"},
		},
	}
	after := Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{ID: "a", Type: "aws.ec2.instance", State: "stopped"},
			{ID: "c", Type: "aws.rds.db_instance", State: "available"},
		},
	}

	report := DiffGraphs(before, after, Query{})

	if report.Added != 1 || report.Removed != 1 || report.Modified != 1 {
		t.Fatalf("unexpected diff counts: %+v", report)
	}
	if len(report.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(report.Changes))
	}
	if report.Changes[0].NodeID != "a" || report.Changes[1].NodeID != "b" || report.Changes[2].NodeID != "c" {
		t.Fatalf("expected deterministic node ordering, got %+v", report.Changes)
	}
}

func TestDiffGraphsSupportsStackFilter(t *testing.T) {
	before := Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{
				ID:    "dev-node",
				Type:  "aws.ec2.instance",
				State: "running",
				Tags: map[string]string{
					"gocools:stack-id":    "dev-stack",
					"gocools:environment": "dev",
				},
			},
			{
				ID:    "prod-node",
				Type:  "aws.ec2.instance",
				State: "running",
				Tags: map[string]string{
					"gocools:stack-id":    "prod-stack",
					"gocools:environment": "prod",
				},
			},
		},
	}

	after := Graph{
		SchemaVersion: SchemaVersion,
		Nodes: []Node{
			{
				ID:    "dev-node",
				Type:  "aws.ec2.instance",
				State: "stopped",
				Tags: map[string]string{
					"gocools:stack-id":    "dev-stack",
					"gocools:environment": "dev",
				},
			},
			{
				ID:    "prod-node",
				Type:  "aws.ec2.instance",
				State: "running",
				Tags: map[string]string{
					"gocools:stack-id":    "prod-stack",
					"gocools:environment": "prod",
				},
			},
		},
	}

	report := DiffGraphs(before, after, Query{
		StackID:     "dev-stack",
		Environment: "dev",
	})

	if report.Modified != 1 || len(report.Changes) != 1 {
		t.Fatalf("expected one filtered change, got %+v", report)
	}
	if report.Changes[0].NodeID != "dev-node" {
		t.Fatalf("expected dev-node in filtered diff, got %+v", report.Changes[0])
	}
}

func TestDiffGraphsLargeSnapshotStress(t *testing.T) {
	before, after := largeGraphPair(4000)

	reportA := DiffGraphs(before, after, Query{})
	reportB := DiffGraphs(before, after, Query{})

	if reportA.Added != 400 || reportA.Removed != 400 || reportA.Modified != 400 {
		t.Fatalf("unexpected stress diff counts: %+v", reportA)
	}

	if len(reportA.Changes) != len(reportB.Changes) {
		t.Fatalf("expected deterministic change length, got A=%d B=%d", len(reportA.Changes), len(reportB.Changes))
	}
	for i := range reportA.Changes {
		if !reflect.DeepEqual(reportA.Changes[i], reportB.Changes[i]) {
			t.Fatalf("expected deterministic change ordering at index %d: A=%+v B=%+v", i, reportA.Changes[i], reportB.Changes[i])
		}
	}
}

func BenchmarkDiffGraphsLarge(b *testing.B) {
	before, after := largeGraphPair(8000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = DiffGraphs(before, after, Query{})
	}
}

func largeGraphPair(nodeCount int) (Graph, Graph) {
	beforeNodes := make([]Node, 0, nodeCount)
	afterNodes := make([]Node, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		id := fmt.Sprintf("node-%05d", i)
		baseNode := Node{
			ID:       id,
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Name:     id,
			State:    "running",
			ARN:      "arn:aws:ec2:us-east-1:123456789012:instance/" + id,
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
			},
			Metadata: map[string]string{
				"instance_type": "t3.micro",
			},
		}
		beforeNodes = append(beforeNodes, baseNode)

		switch {
		case i%10 == 0:
			// removed from "after"
			continue
		case i%10 == 1:
			afterNode := baseNode
			afterNode.State = "stopped"
			afterNodes = append(afterNodes, afterNode)
		default:
			afterNodes = append(afterNodes, baseNode)
		}
	}

	addedCount := nodeCount / 10
	for i := 0; i < addedCount; i++ {
		id := fmt.Sprintf("node-new-%05d", i)
		afterNodes = append(afterNodes, Node{
			ID:       id,
			Type:     "aws.ec2.instance",
			Provider: "aws",
			Region:   "us-east-1",
			Name:     id,
			State:    "running",
			ARN:      "arn:aws:ec2:us-east-1:123456789012:instance/" + id,
			Tags: map[string]string{
				"gocools:stack-id":    "dev-stack",
				"gocools:environment": "dev",
			},
			Metadata: map[string]string{
				"instance_type": "t3.micro",
			},
		})
	}

	before := Graph{
		SchemaVersion: SchemaVersion,
		Nodes:         beforeNodes,
	}
	after := Graph{
		SchemaVersion: SchemaVersion,
		Nodes:         afterNodes,
	}
	return before, after
}
