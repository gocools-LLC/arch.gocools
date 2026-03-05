package apiserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
	"github.com/gocools-LLC/arch.gocools/internal/graph"
)

func TestHealthz(t *testing.T) {
	handler := New(Config{
		Version: "test-version",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	expected := "{\"service\":\"arch\",\"status\":\"ok\",\"version\":\"test-version\"}\n"
	if got := res.Body.String(); got != expected {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestGraphEndpointSupportsStackAndEnvironmentFilters(t *testing.T) {
	graphService := graph.NewService(graph.NewStaticResourceProvider([]model.Resource{
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

	handler := New(Config{
		Version:      "test-version",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		GraphService: graphService,
	}).Handler

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph?stack_id=dev-stack&environment=dev", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var payload graph.Graph
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode graph payload: %v", err)
	}

	if payload.SchemaVersion != graph.SchemaVersion {
		t.Fatalf("expected schema version %q, got %q", graph.SchemaVersion, payload.SchemaVersion)
	}
	if len(payload.Nodes) != 1 {
		t.Fatalf("expected 1 filtered node, got %d", len(payload.Nodes))
	}
	if payload.Nodes[0].ID != "resource-dev" {
		t.Fatalf("expected resource-dev node, got %q", payload.Nodes[0].ID)
	}
}
