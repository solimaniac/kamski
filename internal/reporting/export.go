package reporting

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"kamski/internal/domain"
	"kamski/internal/protocol"
	"kamski/internal/safety"
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
