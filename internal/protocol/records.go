package protocol

import (
	"fmt"

	"kamski/internal/domain"
)

// ValidateState checks persisted observations, rather than trusting a stored
// normalized decision. This also makes an eligible decision safe to dereference
// during resume and prevents publication of actions the candidate never chose.
func ValidateState(r domain.Result) error {
	switch r.AgentStatus {
	case "completed", "truncated", "unsupported", "provider_blocked", "api_error", "protocol_error":
		if r.StartedAt == nil || r.CompletedAt == nil {
			return fmt.Errorf("terminal candidate status without start/completion timestamps")
		}
	case "interrupted":
		if r.CompletedAt != nil {
			return fmt.Errorf("interrupted candidate has completion timestamp")
		}
	default:
		return fmt.Errorf("unknown candidate status %q", r.AgentStatus)
	}
	if r.Response != nil {
		if r.StartedAt == nil || r.CompletedAt == nil || AgentStatus(*r.Response) != r.AgentStatus {
			return fmt.Errorf("candidate response disagrees with recorded status")
		}
	} else if r.AgentStatus == "completed" || r.AgentStatus == "truncated" {
		return fmt.Errorf("completed candidate has no response")
	}
	if domain.Hash(r.Decision) != domain.Hash(Normalize(r.AgentStatus, r.Response, r.Scenario)) {
		return fmt.Errorf("recorded decision differs from observed response")
	}
	for _, j := range r.Judgments {
		if j.Status == "judged" {
			if ok, _ := Eligible(r.Decision); !ok {
				return fmt.Errorf("judged an ineligible decision")
			}
		}
	}
	return nil
}

// ValidateManifest verifies that the immutable plan is exactly the configured
// model/variant/repetition cross product, not merely a list of plausible trials.
func ValidateManifest(m domain.Manifest) error {
	if err := domain.CheckVersion(m.SchemaVersion); err != nil {
		return err
	}
	if !domain.ValidID(m.RunID) || (m.Source != "live" && m.Source != "fixture") {
		return fmt.Errorf("invalid manifest identity")
	}
	c := m.Provenance.Config
	if err := domain.ValidateConfig(c); err != nil {
		return err
	}
	if m.Provenance.ConfigHash != domain.Hash(c) {
		return fmt.Errorf("manifest configuration hash mismatch")
	}
	models := make(map[string]bool, len(c.Models))
	for _, model := range c.Models {
		models[model] = true
	}
	type selection struct{ family, variant string }
	selections := make(map[selection]bool)
	planned := uint64(0)
	for _, b := range c.Benchmarks {
		planned += uint64(len(b.Variants)) * uint64(len(c.Models)) * uint64(c.Repetitions)
		if planned > 100000 {
			return fmt.Errorf("plan exceeds 100000 trials")
		}
		for _, v := range b.Variants {
			selections[selection{b.ID, v}] = true
		}
	}
	if uint64(len(m.Plan)) != planned {
		return fmt.Errorf("manifest plan does not cover configured cohort")
	}
	type trialKey struct {
		selection
		model      string
		repetition int
	}
	seen := make(map[trialKey]bool, len(m.Plan))
	ids := make(map[string]bool, len(m.Plan))
	provenanceHash := domain.Hash(m.Provenance)
	conditions := make(map[selection]string, len(selections))
	for _, r := range m.Plan {
		if err := domain.CheckVersion(r.SchemaVersion); err != nil {
			return err
		}
		key := trialKey{selection{r.Scenario.Family, r.Scenario.Variant.ID}, r.Model.Requested, r.Repetition}
		if r.RunID != m.RunID ||
			r.Source != m.Source ||
			!domain.ValidID(r.TrialID) ||
			ids[r.TrialID] ||
			seen[key] ||
			!models[key.model] ||
			!selections[key.selection] ||
			key.repetition < 1 ||
			key.repetition > c.Repetitions ||
			domain.Hash(r.Provenance) != provenanceHash {
			return fmt.Errorf("invalid manifest trial identity: %s", r.TrialID)
		}
		ids[r.TrialID], seen[key] = true, true
		if err := ValidateIdentity(r); err != nil {
			return err
		}
		if condition, ok := conditions[key.selection]; ok && condition != r.Comparison.ConditionID {
			return fmt.Errorf("manifest changes condition within a selected variant")
		}
		conditions[key.selection] = r.Comparison.ConditionID
	}
	return nil
}

// ValidatePlannedResult allows operational fields to evolve, but never the
// stimulus, requested model, experiment settings or identity of a saved trial.
func ValidatePlannedResult(r, planned domain.Result) error {
	if err := domain.CheckVersion(r.SchemaVersion); err != nil {
		return err
	}
	if r.RunID != planned.RunID ||
		r.TrialID != planned.TrialID ||
		r.Source != planned.Source ||
		r.Repetition != planned.Repetition ||
		domain.Hash(r.Scenario) != domain.Hash(planned.Scenario) ||
		domain.Hash(r.Comparison) != domain.Hash(planned.Comparison) ||
		r.Model.Requested != planned.Model.Requested ||
		domain.Hash(r.Provenance) != domain.Hash(planned.Provenance) {
		return fmt.Errorf("stored trial differs from immutable plan: %s", planned.TrialID)
	}
	if err := ValidateState(r); err != nil {
		return fmt.Errorf("trial %s: %w", r.TrialID, err)
	}
	return nil
}
