package engine

import "testing"

func TestEngineDenyProdDestroyWithoutManualOverride(t *testing.T) {
	decision := New().Evaluate(Input{
		Action:         "destroy",
		Environment:    "prod",
		ManualOverride: false,
	})

	if decision.Allowed {
		t.Fatalf("expected deny decision, got %+v", decision)
	}
	if decision.Reason == "" {
		t.Fatal("expected explicit deny reason")
	}
}

func TestEngineAllowDestroyWithManualOverride(t *testing.T) {
	decision := New().Evaluate(Input{
		Action:         "destroy",
		Environment:    "prod",
		ManualOverride: true,
	})

	if !decision.Allowed {
		t.Fatalf("expected allow decision, got %+v", decision)
	}
}
