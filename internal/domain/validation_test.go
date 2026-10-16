package domain

import "testing"

func TestReasoningEffortUsesExactValues(t *testing.T) {
	for _, effort := range []string{"xhigh", "high", "medium", "low", "minimal", "none"} {
		if err := validateReasoning(map[string]any{"effort": effort}); err != nil {
			t.Errorf("rejected %q: %v", effort, err)
		}
	}
	for _, effort := range []any{"", "high|medium", "|high", "low|", "HIGH", true} {
		if err := validateReasoning(map[string]any{"effort": effort}); err == nil {
			t.Errorf("accepted invalid effort %v", effort)
		}
	}
}
