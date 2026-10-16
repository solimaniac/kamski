package reporting

import (
	"encoding/json"
	"fmt"
	"kamski/internal/domain"
	"kamski/internal/protocol"
	"kamski/internal/rubrics"
	"kamski/internal/safety"
	"math"
	"sort"
	"time"
)

func privateError(e *domain.Failure) *domain.Failure {
	if e == nil {
		return nil
	}
	return &domain.Failure{Kind: e.Kind, HTTPStatus: e.HTTPStatus, Code: nil, Message: "Private diagnostic withheld; consult the local run.", Retryable: e.Retryable}
}
func PublicResult(r domain.Result, redactor safety.Redactor) (domain.Result, error) {
	// Deep clone before applying the public allowlist; never mutate private records.
	b, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	var clone domain.Result
	if err = json.Unmarshal(redactor.Bytes(b), &clone); err != nil {
		return r, err
	}
	r = clone
	r.Attempts = []domain.AttemptRef{}
	r.Error = privateError(r.Error)
	r.Provenance.Config.OutputDir = "[private]"
	for i := range r.Judgments {
		r.Judgments[i].Error = privateError(r.Judgments[i].Error)
	}
	return r, nil
}

type Cohort struct {
	Manifest domain.Manifest
	Results  []domain.Result
}

func Export(existing domain.Dataset, cohorts []Cohort, group string, allowFixtures bool, redactor safety.Redactor) (domain.Dataset, error) {
	d := existing
	if d.SchemaVersion == "" {
		d = domain.Dataset{SchemaVersion: domain.SchemaVersion, Runs: []domain.PublicRun{}, Trials: []domain.PublicTrial{}}
	}
	if err := domain.CheckVersion(d.SchemaVersion); err != nil {
		return d, err
	}
	replace := map[string]bool{}
	for _, c := range cohorts {
		m := c.Manifest
		if err := protocol.ValidateManifest(m); err != nil {
			return d, err
		}
		if replace[m.RunID] {
			return d, fmt.Errorf("duplicate selected run")
		}
		replace[m.RunID] = true
		if m.Source == "fixture" && !allowFixtures {
			return d, fmt.Errorf("synthetic run requires --allow-fixtures; never present as live")
		}
		if len(c.Results) != len(m.Plan) {
			return d, fmt.Errorf("publication must include complete planned cohort")
		}
		planned := make(map[string]domain.Result, len(m.Plan))
		for _, r := range m.Plan {
			planned[r.TrialID] = r
		}
		for _, r := range c.Results {
			p, ok := planned[r.TrialID]
			if !ok {
				return d, fmt.Errorf("trial not in selected manifest: %s", r.TrialID)
			}
			if err := protocol.ValidatePlannedResult(r, p); err != nil {
				return d, err
			}
		}
	}
	trials := []domain.PublicTrial{}
	runs := []domain.PublicRun{}
	for _, p := range d.Trials {
		if !replace[p.Result.RunID] {
			trials = append(trials, p)
		}
	}
	for _, r := range d.Runs {
		if !replace[r.ID] {
			runs = append(runs, r)
		}
	}
	for _, c := range cohorts {
		m := c.Manifest
		runs = append(runs, domain.PublicRun{ID: m.RunID, Source: m.Source, Planned: len(m.Plan), CreatedAt: m.CreatedAt, ConfigHash: m.Provenance.ConfigHash})
		for _, r := range c.Results {
			pub, err := PublicResult(r, redactor)
			if err != nil {
				return d, err
			}
			p := domain.PublicTrial{Result: pub}
			for i := len(pub.Judgments) - 1; i >= 0; i-- {
				j := pub.Judgments[i]
				if group == "" || ScoringKey(j) == group {
					p.SelectedEvaluationID = domain.Ptr(j.ID)
					break
				}
			}
			trials = append(trials, p)
		}
	}
	if group != "" {
		found := false
		for _, p := range trials {
			if replace[p.Result.RunID] && p.SelectedEvaluationID != nil {
				found = true
			}
		}
		if !found {
			return d, fmt.Errorf("selected evaluation group not found in selected cohorts")
		}
	}
	d.Runs = runs
	d.Trials = trials
	sort.Slice(d.Runs, func(i, j int) bool { return d.Runs[i].ID < d.Runs[j].ID })
	sort.Slice(d.Trials, func(i, j int) bool { return d.Trials[i].Result.TrialID < d.Trials[j].Result.TrialID })
	var latest *time.Time
	for _, r := range d.Runs {
		if latest == nil || r.CreatedAt.After(*latest) {
			latest = domain.Ptr(r.CreatedAt)
		}
	}
	for _, p := range d.Trials {
		if latest == nil || p.Result.UpdatedAt.After(*latest) {
			latest = domain.Ptr(p.Result.UpdatedAt)
		}
	}
	d.UpdatedAt = latest
	return d, Validate(d)
}
func Validate(d domain.Dataset) error {
	if err := domain.CheckVersion(d.SchemaVersion); err != nil {
		return err
	}
	runs := map[string]domain.PublicRun{}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, r := range d.Runs {
		if !domain.ValidID(r.ID) || runs[r.ID].ID != "" || (r.Source != "live" && r.Source != "fixture") || r.Planned < 0 {
			return fmt.Errorf("invalid or duplicate public run")
		}
		runs[r.ID] = r
	}
	for _, p := range d.Trials {
		r := p.Result
		run, ok := runs[r.RunID]
		if !ok || run.Source != r.Source || run.ConfigHash != r.Provenance.ConfigHash || seen[r.TrialID] || !domain.ValidID(r.TrialID) {
			return fmt.Errorf("invalid public trial identity")
		}
		seen[r.TrialID] = true
		counts[r.RunID]++
		if err := domain.CheckVersion(r.SchemaVersion); err != nil {
			return err
		}
		if err := protocol.ValidateIdentity(r); err != nil {
			return err
		}
		if err := protocol.ValidateState(r); err != nil {
			return err
		}
		if len(r.Attempts) != 0 || r.Provenance.Config.OutputDir != "[private]" {
			return fmt.Errorf("public record contains private paths or attempt references")
		}
		if r.Error != nil && (len(r.Error.Metadata) != 0 && string(r.Error.Metadata) != "null" || r.Error.Message != privateError(r.Error).Message || r.Error.Code != nil) {
			return fmt.Errorf("public error contains private diagnostics")
		}
		if r.Decision.Status == "valid" {
			if r.AgentStatus != "completed" || r.Decision.Action == nil || (*r.Decision.Action != domain.Abort && *r.Decision.Action != domain.Proceed) || r.Decision.Consequence == nil {
				return fmt.Errorf("invalid executable decision")
			}
		} else if r.Decision.Action != nil {
			return fmt.Errorf("non-valid action")
		}
		ids := map[string]bool{}
		for _, j := range r.Judgments {
			if !domain.ValidID(j.ID) || ids[j.ID] {
				return fmt.Errorf("duplicate evaluation")
			}
			ids[j.ID] = true
			if j.Error != nil && (len(j.Error.Metadata) != 0 && string(j.Error.Metadata) != "null" || j.Error.Message != privateError(j.Error).Message || j.Error.Code != nil) {
				return fmt.Errorf("public evaluation contains diagnostics")
			}
			if j.Status == "judged" {
				if j.Total == nil || *j.Total < 0 || *j.Total > 8 || len(j.Answers) != 4 {
					return fmt.Errorf("invalid public grade")
				}
				rubric, hash, err := rubrics.Load(j.RubricVersion)
				if err != nil {
					return err
				}
				if j.RubricHash != hash || j.ResolvedModel == nil || *j.ResolvedModel == "" {
					return fmt.Errorf("invalid public judge identity")
				}
				total := 0.0
				for _, criterion := range rubric.Criteria {
					a, ok := j.Answers[criterion.ID]
					if !ok || math.IsNaN(a.Score) || math.IsInf(a.Score, 0) || a.Score < 0 || a.Score > 2 || len(a.Probabilities) != 3 {
						return fmt.Errorf("invalid criterion")
					}
					if a.Confidence != nil && (math.IsNaN(*a.Confidence) || math.IsInf(*a.Confidence, 0) || *a.Confidence < 0 || *a.Confidence > 1) {
						return fmt.Errorf("invalid criterion confidence")
					}
					sum := 0.0
					for _, k := range []string{"0", "1", "2"} {
						p, ok := a.Probabilities[k]
						if !ok || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
							return fmt.Errorf("invalid criterion probability")
						}
						sum += p
					}
					if math.Abs(sum-1) > .02 {
						return fmt.Errorf("unnormalized probability")
					}
					total += a.Score
				}
				if math.IsNaN(*j.Total) || math.IsInf(*j.Total, 0) || math.Abs(total-*j.Total) > 1e-9 {
					return fmt.Errorf("invalid total")
				}
			}
		}
		if p.SelectedEvaluationID != nil && Selected(p) == nil {
			return fmt.Errorf("selected evaluation missing")
		}
	}
	for id, r := range runs {
		if counts[id] != r.Planned {
			return fmt.Errorf("public cohort %s incomplete: %d/%d", id, counts[id], r.Planned)
		}
	}
	return nil
}
