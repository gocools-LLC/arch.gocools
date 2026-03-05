package apiserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gocools-LLC/arch.gocools/internal/discovery/model"
	"github.com/gocools-LLC/arch.gocools/internal/graph"
	"github.com/gocools-LLC/arch.gocools/internal/stack/lifecycle"
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

func TestStackOperationsEndpoint(t *testing.T) {
	handler := New(Config{
		Version:      "test-version",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		StackService: lifecycle.NewService(),
	}).Handler

	createBody := `{"action":"create","stack_id":"dev-stack","environment":"dev","actor":"alice","replicas":2}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/operations", bytes.NewBufferString(createBody))
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)

	if createRes.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusOK, createRes.Code, createRes.Body.String())
	}

	destroyBody := `{"action":"destroy","stack_id":"dev-stack","environment":"dev","actor":"alice","confirm":true}`
	destroyReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/operations", bytes.NewBufferString(destroyBody))
	destroyRes := httptest.NewRecorder()
	handler.ServeHTTP(destroyRes, destroyReq)

	if destroyRes.Code != http.StatusOK {
		t.Fatalf("expected destroy status %d, got %d body=%s", http.StatusOK, destroyRes.Code, destroyRes.Body.String())
	}
}

func TestStackDestroyProdRequiresManualOverride(t *testing.T) {
	handler := New(Config{
		Version:      "test-version",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		StackService: lifecycle.NewService(),
	}).Handler

	createBody := `{"action":"create","stack_id":"prod-stack","environment":"prod","actor":"alice"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/operations", bytes.NewBufferString(createBody))
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)

	destroyBody := `{"action":"destroy","stack_id":"prod-stack","environment":"prod","actor":"alice","confirm":true}`
	destroyReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/operations", bytes.NewBufferString(destroyBody))
	destroyRes := httptest.NewRecorder()
	handler.ServeHTTP(destroyRes, destroyReq)

	if destroyRes.Code != http.StatusBadRequest {
		t.Fatalf("expected prod destroy rejection status %d, got %d body=%s", http.StatusBadRequest, destroyRes.Code, destroyRes.Body.String())
	}
}
