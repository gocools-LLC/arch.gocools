package graph

import "testing"

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
