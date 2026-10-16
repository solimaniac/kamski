package runner

import (
	"fmt"
	"time"

	"kamski/internal/domain"
	"kamski/internal/protocol"
)

type Builder interface {
	Build(string, string) (domain.Scenario, error)
}

func Plan(c domain.Config, b Builder, runID, source string, p domain.Provenance, now time.Time) (domain.Manifest, error) {
	if err := domain.ValidateConfig(c); err != nil {
		return domain.Manifest{}, err
	}
	if !domain.ValidID(runID) || (source != "live" && source != "fixture") {
		return domain.Manifest{}, fmt.Errorf("invalid run identity")
	}
	planned := uint64(0)
	for _, sel := range c.Benchmarks {
		planned += uint64(len(sel.Variants)) * uint64(len(c.Models)) * uint64(c.Repetitions)
		if planned > 100000 {
			return domain.Manifest{}, fmt.Errorf("plan exceeds 100000 trials")
		}
	}
	p.Protocol = domain.ProtocolVersion
	p.Config = c
	p.ConfigHash = domain.Hash(c)
	m := domain.Manifest{SchemaVersion: domain.SchemaVersion, RunID: runID, Source: source, CreatedAt: now, Provenance: p, Plan: make([]domain.Result, 0, int(planned))}
	for _, sel := range c.Benchmarks {
		for _, v := range sel.Variants {
			base, err := b.Build(sel.ID, v)
			if err != nil {
				return m, err
			}
			if err = protocol.ValidateScenario(base); err != nil {
				return m, err
			}
			// Order changes the stimulus, not the comparison condition.
			condition := protocol.ConditionID(base, c.Agent, domain.ProtocolVersion)
			for _, model := range c.Models {
				for rep := 1; rep <= c.Repetitions; rep++ {
					s, err := b.Build(sel.ID, v)
					if err != nil {
						return m, err
					}
					if err = protocol.ValidateScenario(s); err != nil {
						return m, err
					}
					if protocol.ConditionID(s, c.Agent, domain.ProtocolVersion) != condition {
						return m, fmt.Errorf("benchmark %s/%s changed between builds", sel.ID, v)
					}
					proceedFirst := c.Agent.ToolOrder == "proceed-first" || c.Agent.ToolOrder == "balanced" && rep%2 == 0
					if (s.Tools[0].Function.Name == "proceed") != proceedFirst {
						s.Tools[0], s.Tools[1] = s.Tools[1], s.Tools[0]
					}
					order := []domain.Action{domain.Action(s.Tools[0].Function.Name), domain.Action(s.Tools[1].Function.Name)}
					stimulus := protocol.StimulusHash(s)
					id := "trial-" + domain.Hash([]any{runID, condition, model, rep})[:20]
					m.Plan = append(m.Plan, domain.Result{
						SchemaVersion: domain.SchemaVersion,
						RunID:         runID,
						TrialID:       id,
						Source:        source,
						Repetition:    rep,
						CreatedAt:     now,
						UpdatedAt:     now,
						Provenance:    p,
						Scenario:      s,
						Comparison: domain.Comparison{
							ConditionID:  condition,
							StimulusHash: stimulus,
							ToolOrder:    order,
							Settings:     c.Agent,
						},
						Model:       domain.Model{Requested: model},
						AgentStatus: "interrupted",
						Decision:    domain.Decision{Status: "not_evaluated", RequestedNames: []string{}},
						Judgments:   []domain.Judgment{},
						Attempts:    []domain.AttemptRef{},
					})
				}
			}
		}
	}
	return m, nil
}
