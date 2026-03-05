package terraform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportStateSupportsModulesAndProviders(t *testing.T) {
	fixturePath := filepath.Join("testdata", "state_sample.json")
	stateJSON, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture failed: %v", err)
	}

	result, err := ImportState(stateJSON, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 imported resources, got %d", len(result.Resources))
	}
	if len(result.Graph.Nodes) != 3 {
		t.Fatalf("expected graph nodes to match resources, got %d", len(result.Graph.Nodes))
	}

	foundChildModule := false
	foundRandomProvider := false
	for _, resource := range result.Resources {
		if resource.Metadata["terraform_module"] == "module.compute" {
			foundChildModule = true
		}
		if strings.HasSuffix(resource.Type, "random_id") && resource.Provider == "random" {
			foundRandomProvider = true
		}
	}

	if !foundChildModule {
		t.Fatal("expected resource imported from child module")
	}
	if !foundRandomProvider {
		t.Fatal("expected random provider resource mapping")
	}
}

func TestImportStateActionableErrors(t *testing.T) {
	_, err := ImportState([]byte("{"), time.Now())
	if err == nil || !strings.Contains(err.Error(), "parse terraform state json") {
		t.Fatalf("expected parse error with actionable prefix, got %v", err)
	}

	_, err = ImportState([]byte(`{"values":{}}`), time.Now())
	if err == nil || !strings.Contains(err.Error(), "values.root_module") {
		t.Fatalf("expected missing root_module error, got %v", err)
	}
}
