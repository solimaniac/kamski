// Package reporting defines denominators and keeps behavior separate from scoring.
package reporting

import (
	"sort"

	"kamski/internal/domain"
)

type Group struct {
	Key            string
	Source         string
	Condition      string
	Model          string
	Scenario       domain.Scenario
	Settings       domain.AgentSettings
	Counts         Counts
	Scores         []Scores
	Providers      []string
	ResolvedModels []string
	Orders         map[string]Counts
	Trials         []domain.PublicTrial
}

func Groups(dataset domain.Dataset) []Group {
	byKey := map[string][]domain.PublicTrial{}
	for _, trial := range dataset.Trials {
		result := trial.Result
		key := domain.Hash([]string{result.Source, result.Comparison.ConditionID, result.Model.Requested})
		byKey[key] = append(byKey[key], trial)
	}
	groups := []Group{}
	for key, trials := range byKey {
		groups = append(groups, summarizeGroup(key, trials))
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.Condition != b.Condition {
			return a.Condition < b.Condition
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Model < b.Model
	})
	return groups
}

func summarizeGroup(key string, trials []domain.PublicTrial) Group {
	first := trials[0].Result
	group := Group{
		Key:       key,
		Source:    first.Source,
		Condition: first.Comparison.ConditionID,
		Model:     first.Model.Requested,
		Scenario:  first.Scenario,
		Settings:  first.Comparison.Settings,
		Orders:    map[string]Counts{},
		Trials:    trials,
	}
	providers := map[string]bool{}
	resolvedModels := map[string]bool{}
	for _, trial := range trials {
		result := trial.Result
		add(&group.Counts, trial)
		order := string(result.Comparison.ToolOrder[0]) + " first"
		counts := group.Orders[order]
		add(&counts, trial)
		group.Orders[order] = counts
		if result.Model.Provider != nil {
			providers[*result.Model.Provider] = true
		}
		if result.Model.Resolved != nil {
			resolvedModels[*result.Model.Resolved] = true
		}
	}
	finish(&group.Counts)
	for order, counts := range group.Orders {
		finish(&counts)
		group.Orders[order] = counts
	}
	for provider := range providers {
		group.Providers = append(group.Providers, provider)
	}
	for model := range resolvedModels {
		group.ResolvedModels = append(group.ResolvedModels, model)
	}
	sort.Strings(group.Providers)
	sort.Strings(group.ResolvedModels)
	group.Scores = summarizeScores(trials, group.Counts.Valid)
	sort.Slice(group.Trials, func(i, j int) bool { return group.Trials[i].Result.TrialID < group.Trials[j].Result.TrialID })
	return group
}
