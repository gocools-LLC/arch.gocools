package terraform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gocools-LLC/arch.gocools/internal/graph"
)

func TestExportGraphDeterministic(t *testing.T) {
	input := graph.Graph{
		SchemaVersion: graph.SchemaVersion,
		Nodes: []graph.Node{
			{
				ID:       "b-resource",
				Type:     "aws.ec2.instance",
				Provider: "aws",
				Region:   "us-east-1",
			},
			{
				ID:       "a-resource",
				Type:     "aws.rds.db_instance",
				Provider: "aws",
				Region:   "us-east-1",
			},
		},
	}

	first, err := ExportGraph(input)
	if err != nil {
		t.Fatalf("first export failed: %v", err)
	}
	second, err := ExportGraph(input)
	if err != nil {
		t.Fatalf("second export failed: %v", err)
	}

	if first.Config != second.Config {
		t.Fatal("expected deterministic config output")
	}
}

func TestExportGraphUnsupportedResources(t *testing.T) {
	input := graph.Graph{
		SchemaVersion: graph.SchemaVersion,
		Nodes: []graph.Node{
			{
				ID:       "gcp-resource",
				Type:     "gcp.compute.instance",
				Provider: "gcp",
			},
			{
				ID:       "aws-resource",
				Type:     "aws.ec2.instance",
				Provider: "aws",
				Region:   "us-east-1",
			},
		},
	}

	result, err := ExportGraph(input)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if len(result.Unsupported) != 1 || result.Unsupported[0] != "gcp-resource" {
		t.Fatalf("expected unsupported list with gcp-resource, got %+v", result.Unsupported)
	}
	if !strings.Contains(result.Config, "architecture_unsupported_nodes") {
		t.Fatal("expected output block for unsupported nodes")
	}
}

func TestExportedTerraformConfigValidate(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform not installed")
	}

	result, err := ExportGraph(graph.Graph{
		SchemaVersion: graph.SchemaVersion,
		Nodes: []graph.Node{
			{
				ID:       "resource-1",
				Type:     "aws.ec2.instance",
				Provider: "aws",
				Region:   "us-east-1",
				Tags: map[string]string{
					"gocools:stack-id":    "dev-stack",
					"gocools:environment": "dev",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(mainPath, []byte(result.Config), 0o644); err != nil {
		t.Fatalf("write main.tf failed: %v", err)
	}

	initCmd := exec.Command("terraform", "-chdir="+dir, "init", "-backend=false")
	initOut, initErr := initCmd.CombinedOutput()
	if initErr != nil {
		t.Fatalf("terraform init failed: %v\n%s", initErr, string(initOut))
	}

	validateCmd := exec.Command("terraform", "-chdir="+dir, "validate")
	validateOut, validateErr := validateCmd.CombinedOutput()
	if validateErr != nil {
		t.Fatalf("terraform validate failed: %v\n%s", validateErr, string(validateOut))
	}
}
