package engine

import "strings"

type Input struct {
	Action         string
	Environment    string
	Confirm        bool
	ManualOverride bool
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	RuleID  string `json:"rule_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Evaluate(input Input) Decision {
	action := strings.ToLower(strings.TrimSpace(input.Action))
	environment := strings.ToLower(strings.TrimSpace(input.Environment))

	if action == "destroy" && environment == "prod" && !input.ManualOverride {
		return Decision{
			Allowed: false,
			RuleID:  "deny.prod.destroy.without-manual-override",
			Reason:  "policy deny: production destroy requires manual_override=true",
		}
	}

	return Decision{
		Allowed: true,
	}
}
