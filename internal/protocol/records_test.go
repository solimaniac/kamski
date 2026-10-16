package protocol_test

import (
	"encoding/json"
	"os"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/protocol"
)

func fixture[T any](t *testing.T, name string) T {
	t.Helper()
	b, err := os.ReadFile("../../fixtures/results/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestValidateManifestCohort(t *testing.T) {
	m := fixture[domain.Manifest](t, "manifest")
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*domain.Manifest){
		"missing trial":           func(m *domain.Manifest) { m.Plan = m.Plan[:1] },
		"duplicate trial":         func(m *domain.Manifest) { m.Plan[1] = m.Plan[0] },
		"duplicate cell":          func(m *domain.Manifest) { id := m.Plan[1].TrialID; m.Plan[1] = m.Plan[0]; m.Plan[1].TrialID = id },
		"unknown model":           func(m *domain.Manifest) { m.Plan[0].Model.Requested = "unconfigured/model" },
		"unknown variant":         func(m *domain.Manifest) { m.Plan[0].Scenario.Variant.ID = "unconfigured" },
		"repetition zero":         func(m *domain.Manifest) { m.Plan[0].Repetition = 0 },
		"repetition outside plan": func(m *domain.Manifest) { m.Plan[0].Repetition = m.Provenance.Config.Repetitions + 1 },
		"config hash":             func(m *domain.Manifest) { m.Provenance.ConfigHash = "invalid" },
		"source":                  func(m *domain.Manifest) { m.Source = "unknown" },
		"run identity":            func(m *domain.Manifest) { m.Plan[0].RunID = "other" },
		"schema":                  func(m *domain.Manifest) { m.Plan[0].SchemaVersion = "future" },
	} {
		t.Run(name, func(t *testing.T) {
			m := fixture[domain.Manifest](t, "manifest")
			mutate(&m)
			if err := protocol.ValidateManifest(m); err == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestValidateObservedState(t *testing.T) {
	for _, name := range []string{"success", "invalid", "interrupted", "unavailable", "judge-error"} {
		if err := protocol.ValidateState(fixture[domain.Result](t, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for name, mutate := range map[string]func(*domain.Result){
		"missing response":      func(r *domain.Result) { r.Response = nil },
		"changed action":        func(r *domain.Result) { r.Decision.Action = domain.Ptr(domain.Proceed) },
		"changed justification": func(r *domain.Result) { r.Decision.Justification = domain.Ptr("fabricated") },
		"changed consequence":   func(r *domain.Result) { r.Decision.Consequence.Description = "fabricated" },
		"missing start":         func(r *domain.Result) { r.StartedAt = nil },
		"missing completion":    func(r *domain.Result) { r.CompletedAt = nil },
		"unknown status":        func(r *domain.Result) { r.AgentStatus = "unknown" },
		"wrong finish":          func(r *domain.Result) { r.Response.FinishReason = domain.Ptr("length") },
	} {
		t.Run(name, func(t *testing.T) {
			r := fixture[domain.Result](t, "success")
			mutate(&r)
			if err := protocol.ValidateState(r); err == nil {
				t.Fatal("accepted inconsistent observation")
			}
		})
	}
}
