package lifecycle

import (
	"strings"
	"testing"
	"time"
)

func TestLifecycleCreateScaleUpdateDestroy(t *testing.T) {
	service := NewService()
	service.now = func() time.Time { return time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC) }

	createResult, err := service.Apply(Request{
		Action:      ActionCreate,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Replicas:    2,
		Tags: map[string]string{
			"gocools:stack-id":    "dev-stack",
			"gocools:environment": "dev",
			"gocools:owner":       "alice",
		},
		Metadata: map[string]string{
			"owner": "team-a",
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !createResult.Executed || createResult.Stack == nil || createResult.Stack.Replicas != 2 {
		t.Fatalf("unexpected create result: %+v", createResult)
	}

	scaleResult, err := service.Apply(Request{
		Action:      ActionScale,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Replicas:    4,
	})
	if err != nil {
		t.Fatalf("scale failed: %v", err)
	}
	if scaleResult.Stack == nil || scaleResult.Stack.Replicas != 4 {
		t.Fatalf("expected scaled replicas 4, got %+v", scaleResult.Stack)
	}

	updateResult, err := service.Apply(Request{
		Action:      ActionUpdate,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Metadata: map[string]string{
			"purpose": "integration-test",
		},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updateResult.Stack == nil || updateResult.Stack.Metadata["purpose"] != "integration-test" {
		t.Fatalf("unexpected update metadata: %+v", updateResult.Stack)
	}

	destroyResult, err := service.Apply(Request{
		Action:      ActionDestroy,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Confirm:     true,
	})
	if err != nil {
		t.Fatalf("destroy failed: %v", err)
	}
	if !destroyResult.Executed {
		t.Fatalf("expected destroy executed result, got %+v", destroyResult)
	}
	if _, exists := service.Stack("dev-stack"); exists {
		t.Fatal("expected stack to be deleted")
	}
}

func TestDestroyProdRequiresManualOverride(t *testing.T) {
	service := NewService()
	_, err := service.Apply(Request{
		Action:      ActionCreate,
		StackID:     "prod-stack",
		Environment: "prod",
		Actor:       "alice",
		Tags: map[string]string{
			"gocools:stack-id":    "prod-stack",
			"gocools:environment": "prod",
			"gocools:owner":       "alice",
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = service.Apply(Request{
		Action:      ActionDestroy,
		StackID:     "prod-stack",
		Environment: "prod",
		Actor:       "alice",
		Confirm:     true,
	})
	if err == nil {
		t.Fatal("expected prod destroy to require manual override")
	}
	if !strings.Contains(err.Error(), "policy deny") {
		t.Fatalf("expected explicit policy deny reason, got %v", err)
	}

	_, err = service.Apply(Request{
		Action:         ActionDestroy,
		StackID:        "prod-stack",
		Environment:    "prod",
		Actor:          "alice",
		Confirm:        true,
		ManualOverride: true,
	})
	if err != nil {
		t.Fatalf("destroy with manual override failed: %v", err)
	}
}

func TestDryRunDoesNotMutateStateAndAuditIncludesActorStack(t *testing.T) {
	service := NewService()
	_, err := service.Apply(Request{
		Action:      ActionCreate,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Tags: map[string]string{
			"gocools:stack-id":    "dev-stack",
			"gocools:environment": "dev",
			"gocools:owner":       "alice",
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	result, err := service.Apply(Request{
		Action:      ActionDestroy,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "bob",
		Confirm:     true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("dry-run destroy failed: %v", err)
	}
	if result.Executed {
		t.Fatalf("expected dry-run executed=false, got %+v", result)
	}
	if _, exists := service.Stack("dev-stack"); !exists {
		t.Fatal("expected stack to remain after dry-run")
	}

	logs := service.AuditLogs()
	if len(logs) == 0 {
		t.Fatal("expected audit logs")
	}
	last := logs[len(logs)-1]
	if last.Actor != "bob" || last.StackID != "dev-stack" {
		t.Fatalf("expected audit actor/stack in log, got %+v", last)
	}
}

func TestCreateFailsWithoutRequiredTags(t *testing.T) {
	service := NewService()

	_, err := service.Apply(Request{
		Action:      ActionCreate,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Tags: map[string]string{
			"gocools:stack-id": "dev-stack",
		},
	})
	if err == nil {
		t.Fatal("expected required tag validation error")
	}
}

func TestUpdateFailsWhenOwnerTagMissing(t *testing.T) {
	service := NewService()
	service.stacks["dev-stack"] = Stack{
		ID:          "dev-stack",
		Environment: "dev",
		Replicas:    1,
		Tags: map[string]string{
			"gocools:stack-id":    "dev-stack",
			"gocools:environment": "dev",
		},
	}

	_, err := service.Apply(Request{
		Action:      ActionUpdate,
		StackID:     "dev-stack",
		Environment: "dev",
		Actor:       "alice",
		Metadata: map[string]string{
			"purpose": "upgrade",
		},
	})
	if err == nil {
		t.Fatal("expected owner-tag validation error on update")
	}
	if !strings.Contains(err.Error(), "gocools:owner") || !strings.Contains(err.Error(), "remediation") {
		t.Fatalf("expected owner-tag remediation message, got %v", err)
	}
}

func TestDestroyFailsWhenOwnerTagMissing(t *testing.T) {
	service := NewService()
	service.stacks["dev-stack"] = Stack{
		ID:          "dev-stack",
		Environment: "dev",
		Replicas:    1,
		Tags: map[string]string{
			"gocools:stack-id":    "dev-stack",
			"gocools:environment": "dev",
		},
	}

	for _, dryRun := range []bool{false, true} {
		_, err := service.Apply(Request{
			Action:      ActionDestroy,
			StackID:     "dev-stack",
			Environment: "dev",
			Actor:       "alice",
			Confirm:     true,
			DryRun:      dryRun,
		})
		if err == nil {
			t.Fatalf("expected owner-tag validation error on destroy (dry_run=%v)", dryRun)
		}
		if !strings.Contains(err.Error(), "gocools:owner") || !strings.Contains(err.Error(), "remediation") {
			t.Fatalf("expected owner-tag remediation message on destroy (dry_run=%v), got %v", dryRun, err)
		}
	}
}
