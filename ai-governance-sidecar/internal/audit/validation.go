package audit

import (
	"encoding/json"
	"fmt"
)

func validateLogInput(toolInput json.RawMessage, decision Decision, reason string) error {
	if len(toolInput) == 0 {
		return fmt.Errorf("tool_input cannot be empty")
	}

	if !json.Valid(toolInput) {
		return fmt.Errorf("tool_input must be valid JSON")
	}

	if !isValidDecision(decision) {
		return fmt.Errorf("invalid decision: %s", decision)
	}

	if reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}

	return nil
}

func isValidDecision(d Decision) bool {
	return d == DecisionAllow || d == DecisionDeny
}