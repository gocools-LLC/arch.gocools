package lifecycle

import (
	"errors"
	"sync"
	"time"
)

type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionScale   Action = "scale"
	ActionDestroy Action = "destroy"
)

type Stack struct {
	ID          string            `json:"id"`
	Environment string            `json:"environment"`
	Replicas    int               `json:"replicas"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Request struct {
	Action         Action            `json:"action"`
	StackID        string            `json:"stack_id"`
	Environment    string            `json:"environment"`
	Actor          string            `json:"actor"`
	Replicas       int               `json:"replicas,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Confirm        bool              `json:"confirm,omitempty"`
	DryRun         bool              `json:"dry_run,omitempty"`
	ManualOverride bool              `json:"manual_override,omitempty"`
}

type Result struct {
	Executed bool       `json:"executed"`
	DryRun   bool       `json:"dry_run"`
	Message  string     `json:"message"`
	Stack    *Stack     `json:"stack,omitempty"`
	Audit    AuditEntry `json:"audit"`
}

type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Actor       string    `json:"actor"`
	StackID     string    `json:"stack_id"`
	Environment string    `json:"environment"`
	Action      Action    `json:"action"`
	DryRun      bool      `json:"dry_run"`
	Result      string    `json:"result"`
}

type Service struct {
	mu        sync.RWMutex
	stacks    map[string]Stack
	auditLogs []AuditEntry
	now       func() time.Time
}

func NewService() *Service {
	return &Service{
		stacks:    map[string]Stack{},
		auditLogs: []AuditEntry{},
		now:       time.Now,
	}
}

func (s *Service) Apply(request Request) (Result, error) {
	if request.StackID == "" {
		return Result{}, errors.New("stack_id is required")
	}
	if request.Environment == "" {
		return Result{}, errors.New("environment is required")
	}
	if request.Actor == "" {
		return Result{}, errors.New("actor is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stack, exists := s.stacks[request.StackID]
	if exists && stack.Environment != request.Environment {
		return Result{}, errors.New("stack environment mismatch")
	}

	switch request.Action {
	case ActionCreate:
		if exists {
			return Result{}, errors.New("stack already exists")
		}

		newStack := Stack{
			ID:          request.StackID,
			Environment: request.Environment,
			Replicas:    defaultReplicas(request.Replicas),
			Metadata:    cloneMap(request.Metadata),
		}

		if !request.DryRun {
			s.stacks[request.StackID] = newStack
		}

		return s.successResult(request, "stack created", &newStack)

	case ActionUpdate:
		if !exists {
			return Result{}, errors.New("stack does not exist")
		}

		updated := stack
		for key, value := range request.Metadata {
			if updated.Metadata == nil {
				updated.Metadata = map[string]string{}
			}
			updated.Metadata[key] = value
		}

		if !request.DryRun {
			s.stacks[request.StackID] = updated
		}

		return s.successResult(request, "stack updated", &updated)

	case ActionScale:
		if !exists {
			return Result{}, errors.New("stack does not exist")
		}
		if request.Replicas <= 0 {
			return Result{}, errors.New("replicas must be greater than zero")
		}

		scaled := stack
		scaled.Replicas = request.Replicas
		if !request.DryRun {
			s.stacks[request.StackID] = scaled
		}

		return s.successResult(request, "stack scaled", &scaled)

	case ActionDestroy:
		if !exists {
			return Result{}, errors.New("stack does not exist")
		}
		if !request.Confirm {
			return Result{}, errors.New("destroy requires confirm=true")
		}
		if request.Environment == "prod" && !request.ManualOverride {
			return Result{}, errors.New("destroy in prod requires manual_override=true")
		}

		destroyed := stack
		if !request.DryRun {
			delete(s.stacks, request.StackID)
		}

		return s.successResult(request, "stack destroyed", &destroyed)

	default:
		return Result{}, errors.New("unsupported action")
	}
}

func (s *Service) Stack(id string) (Stack, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stack, exists := s.stacks[id]
	return stack, exists
}

func (s *Service) AuditLogs() []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logs := make([]AuditEntry, len(s.auditLogs))
	copy(logs, s.auditLogs)
	return logs
}

func (s *Service) successResult(request Request, message string, stack *Stack) (Result, error) {
	audit := AuditEntry{
		Timestamp:   s.now().UTC(),
		Actor:       request.Actor,
		StackID:     request.StackID,
		Environment: request.Environment,
		Action:      request.Action,
		DryRun:      request.DryRun,
		Result:      "success",
	}
	s.auditLogs = append(s.auditLogs, audit)

	var stackCopy *Stack
	if stack != nil {
		copyValue := *stack
		copyValue.Metadata = cloneMap(copyValue.Metadata)
		stackCopy = &copyValue
	}

	return Result{
		Executed: !request.DryRun,
		DryRun:   request.DryRun,
		Message:  message,
		Stack:    stackCopy,
		Audit:    audit,
	}, nil
}

func defaultReplicas(replicas int) int {
	if replicas <= 0 {
		return 1
	}
	return replicas
}

func cloneMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
