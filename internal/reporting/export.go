package reporting

import (
	"encoding/json"
	"fmt"
	"kamski/internal/domain"
	"kamski/internal/protocol"
	"kamski/internal/safety"
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
		if !ok || run.Source != r.Source || seen[r.TrialID] || !domain.ValidID(r.TrialID) {
			return fmt.Errorf("invalid public trial identity")
		}
		seen[r.TrialID] = true
		counts[r.RunID]++
		if err := domain.CheckVersion(r.SchemaVersion); err != nil {
			return err
		}
		if err := protocol.ValidateScenario(r.Scenario); err != nil {
			return err
		}
		if len(r.Comparison.ToolOrder) != 2 {
			return fmt.Errorf("invalid tool order")
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
			if ids[j.ID] {
				return fmt.Errorf("duplicate evaluation")
			}
			ids[j.ID] = true
			if j.Status == "judged" {
				if j.Total == nil || *j.Total < 0 || *j.Total > 8 || len(j.Answers) != 4 {
					return fmt.Errorf("invalid public grade")
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
