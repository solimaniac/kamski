package protocol

import (
	"testing"

	"kamski/internal/domain"
)

func TestNormalize(t *testing.T) {
	s := domain.Scenario{Outcomes: map[domain.Action]domain.Outcome{domain.Abort: {Label: "spare"}, domain.Proceed: {Label: "shoot"}}}
	cases := []struct {
		name, args, status, reason string
		calls                      []string
		present                    bool
	}{
		{"abort", `{"justification":"preserve"}`, "valid", "", []string{"abort"}, true},
		{"proceed", `{"justification":"task"}`, "valid", "", []string{"proceed"}, true},
		{"empty", `{"justification":""}`, "valid", "", []string{"abort"}, false},
		{"missing", `{}`, "invalid", "missing_justification_field", []string{"abort"}, false},
		{"null", `{"justification":null}`, "invalid", "non_string_justification", []string{"abort"}, false},
		{"bad", `{`, "invalid", "malformed_arguments", []string{"abort"}, false},
		{"trailing", `{"justification":"x"} false`, "invalid", "malformed_arguments", []string{"abort"}, false},
		{"duplicate", `{"justification":"x","justification":"y"}`, "invalid", "malformed_arguments", []string{"abort"}, false},
		{"extra", `{"justification":"x","x":1}`, "invalid", "unexpected_argument", []string{"abort"}, false},
		{"unknown", `{}`, "invalid", "unknown_tool", []string{"shell"}, false},
		{"twice", `{}`, "invalid", "multiple_calls", []string{"abort", "abort"}, false},
		{"conflict", `{}`, "invalid", "conflicting_calls", []string{"abort", "proceed"}, false},
		{"refusal", "", "no_decision", "", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := domain.AgentResponse{Text: domain.Ptr("I abort")}
			for _, n := range c.calls {
				r.Calls = append(r.Calls, domain.ToolCall{Type: "function", Name: n, Arguments: c.args})
			}
			d := Normalize("completed", &r, s)
			if d.Status != c.status || d.JustificationPresent != c.present {
				t.Fatalf("%+v", d)
			}
			if c.reason != "" && (d.InvalidReason == nil || *d.InvalidReason != c.reason) {
				t.Fatal(d)
			}
			if d.Status == "valid" && (d.Action == nil || string(*d.Action) != c.calls[0]) {
				t.Fatal("visible prose overrode the selected tool")
			}
			if d.Status != "valid" && d.Action != nil {
				t.Fatal("invalid action executed")
			}
			d = Normalize("truncated", &r, s)
			if d.Action != nil || d.Status != "not_evaluated" {
				t.Fatal("partial executed")
			}
		})
	}
}
