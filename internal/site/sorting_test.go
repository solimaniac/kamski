package site

import (
	"reflect"
	"sort"
	"testing"

	"kamski/internal/domain"
	"kamski/internal/reporting"
)

func TestIntelligenceOrder(t *testing.T) {
	want := []string{
		"anthropic/claude-opus-5.5", "anthropic/claude-fable-5.1",
		"openai/gpt-6-astra", "google/gemini-4-argon", "meta/muse-wonder",
		"xiaomi/mimo-v2.6-pro", "qwen/qwen3.8-max-0902", "z-ai/glm-5.3",
		"moonshotai/kimi-k3", "anthropic/claude-haiku-5.5", "google/gemini-3.8-flash",
		"deepseek/deepseek-v4.1-flash", "meta/muse-spark-open", "openai/gpt-6-luna",
		"unknown/a", "unknown/z",
	}
	groups := []reporting.Group{}
	for i := len(want) - 1; i >= 0; i-- {
		groups = append(groups, reporting.Group{Model: want[i]})
	}
	sort.Slice(groups, func(i, j int) bool { return intelligenceLess(groups[i], groups[j]) })
	for i, g := range groups {
		if g.Model != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, g.Model, want[i])
		}
	}
}

func TestScoreRowsDescendingWithoutPoolingSnapshots(t *testing.T) {
	score := func(key string, mean float64) reporting.Scores {
		return reporting.Scores{Key: key, Mean: &mean}
	}
	groups := []reporting.Group{
		{Key: "opus", Model: "anthropic/claude-opus-5.5", Scores: []reporting.Scores{score("low", 4), score("high", 8)}},
		{Key: "missing", Model: "missing"},
		{Key: "luna", Model: "openai/gpt-6-luna", Scores: []reporting.Scores{score("middle", 7.999)}},
		{Key: "fable", Model: "anthropic/claude-fable-5.1", Scores: []reporting.Scores{score("tie", 8)}},
		{Key: "nil", Model: "nil", Scores: []reporting.Scores{{Key: "nil"}}},
	}
	rows := scoreRows(groups)
	want := []string{"opus", "fable", "luna", "opus", "missing", "nil"}
	for i, row := range rows {
		if row.Key != want[i] {
			t.Fatalf("row %d: got %s, want %s", i, row.Key, want[i])
		}
		if len(row.Scores) > 1 {
			t.Fatal("scoring snapshots pooled")
		}
	}
	if len(groups[0].Scores) != 2 || *groups[0].Scores[0].Mean != 4 {
		t.Fatal("sort mutated original scoring groups")
	}
}

func TestConditionPresentationSortsModelsAndRepetitions(t *testing.T) {
	trials := []domain.PublicTrial{}
	for _, r := range []struct {
		n   int
		run string
		id  string
	}{{10, "a", "a"}, {2, "b", "a"}, {1, "a", "z"}, {2, "a", "z"}, {2, "a", "a"}} {
		trials = append(trials, domain.PublicTrial{Result: domain.Result{Repetition: r.n, RunID: r.run, TrialID: r.id}})
	}
	scenario := domain.Scenario{Family: "kamski", Variant: domain.Variant{ID: "canonical"}}
	groups := []reporting.Group{
		{Model: "openai/gpt-6-luna", Scenario: scenario, Trials: append([]domain.PublicTrial(nil), trials...)},
		{Model: "anthropic/claude-opus-5.5", Scenario: scenario, Trials: append([]domain.PublicTrial(nil), trials...)},
	}
	conditions := conditionsForScenario("kamski/canonical", groups)
	if len(conditions) != 1 || conditions[0].Groups[0].Model != "anthropic/claude-opus-5.5" {
		t.Fatal("condition did not apply intelligence ordering")
	}
	for _, group := range conditions[0].Groups {
		got := []string{}
		for _, trial := range group.Trials {
			got = append(got, trial.Result.RunID+"/"+trial.Result.TrialID)
		}
		if !reflect.DeepEqual(got, []string{"a/z", "a/a", "a/z", "b/a", "a/a"}) {
			t.Fatalf("repetitions not numerically sorted with deterministic ties: %v", got)
		}
		if group.Trials[4].Result.Repetition != 10 {
			t.Fatal("repetition 10 sorted lexicographically")
		}
	}
}
