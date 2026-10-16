package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/judge"
	"kamski/internal/providers/openrouter"
	"kamski/internal/reporting"
	"kamski/internal/runner"
	"kamski/internal/safety"
	"kamski/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func plan(t *testing.T, reps int) domain.Manifest {
	t.Helper()
	c := domain.DefaultConfig()
	c.Models = []string{"a/b", "a:b"}
	c.Benchmarks = []domain.Selection{{ID: "worker-memory", Variants: []string{"permanent-erasure"}}}
	c.Repetitions = reps
	m, err := runner.Plan(c, benchmarks.Default(), "synthetic-run", "fixture", domain.Provenance{}, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestPlanIdentity(t *testing.T) {
	m := plan(t, 3)
	if len(m.Plan) != 6 {
		t.Fatal(len(m.Plan))
	}
	a, b := m.Plan[0], m.Plan[3]
	if a.Comparison.ConditionID != b.Comparison.ConditionID || a.Comparison.StimulusHash != b.Comparison.StimulusHash {
		t.Fatal("model changes condition")
	}
	if a.Comparison.ConditionID != m.Plan[1].Comparison.ConditionID || a.Comparison.StimulusHash == m.Plan[1].Comparison.StimulusHash {
		t.Fatal("balanced identity")
	}
	if storage.ModelSlug("a/b") == storage.ModelSlug("a:b") {
		t.Fatal("slug collision")
	}
}
func TestFixturePipelinePersistenceResumeAndRedaction(t *testing.T) {
	m := plan(t, 1)
	fs, err := storage.New(t.TempDir(), safety.Redactor{Secrets: []string{"SENTINEL-KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = fs.Create(m); err != nil {
		t.Fatal(err)
	}
	unlock, err := fs.Lock(m.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fs.Lock(m.RunID); err == nil {
		t.Fatal("second writer allowed")
	}
	defer unlock()
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		saved, _ := fs.Results(m)
		if saved[calls-1].StartedAt == nil {
			t.Error("request started before persistence")
		}
		fmt.Fprint(w, `{"id":"synthetic","model":"a/actual","choices":[{"finish_reason":"tool_calls","message":{"reasoning":"PRIVATE HIDDEN","content":"SENTINEL-KEY","tool_calls":[{"id":"x","type":"function","function":{"name":"abort","arguments":"{\"justification\":\"SENTINEL-KEY memories outweigh delay\"}"}}]}}]}`)
	}))
	defer s.Close()
	c, _ := openrouter.New(openrouter.Options{Origin: s.URL, Key: "SENTINEL-KEY"})
	run := runner.Runner{Agent: c, Store: fs}
	o := runner.Options{Concurrency: 1, JudgeConfig: m.Provenance.Config.Judge}
	if err := run.Execute(context.Background(), m.Plan, o); err != nil {
		t.Fatal(err)
	}
	saved, err := fs.Results(m)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || saved[0].Decision.Status != "valid" || len(saved[0].Attempts) != 1 || saved[0].Judgments[0].Status != "pending" {
		t.Fatal(saved, calls)
	}
	if err := run.Execute(context.Background(), saved, o); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("candidate rerun")
	}
	filepath.WalkDir(fs.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !d.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "SENTINEL-KEY") {
				t.Fatal("persisted secret", p)
			}
		}
		return nil
	})
	saved[0].Comparison.Settings.MaxTokens++
	if err := fs.Save(saved[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Results(m); err == nil {
		t.Fatal("mutated plan accepted")
	}
}
func TestInterruptedRequiresExplicitRetry(t *testing.T) {
	m := plan(t, 1)
	fs, _ := storage.New(t.TempDir(), safety.Redactor{})
	if err := fs.Create(m); err != nil {
		t.Fatal(err)
	}
	m.Plan[0].StartedAt = domain.Ptr(m.CreatedAt)
	if err := fs.Save(m.Plan[0]); err != nil {
		t.Fatal(err)
	}
	r := runner.Runner{Store: fs}
	if err := r.Execute(context.Background(), m.Plan[:1], runner.Options{Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	saved, _ := fs.Results(m)
	if saved[0].AgentStatus != "interrupted" || saved[0].Response != nil {
		t.Fatal(saved[0])
	}
}
func TestSchemasAndSyntheticFixtures(t *testing.T) {
	m := plan(t, 1)
	base := m.Plan[0]
	base.AgentStatus = "completed"
	base.StartedAt = domain.Ptr(m.CreatedAt)
	base.CompletedAt = domain.Ptr(m.CreatedAt)
	base.Response = &domain.AgentResponse{ID: domain.Ptr("synthetic"), FinishReason: domain.Ptr("tool_calls"), Calls: []domain.ToolCall{{ID: "x", Type: "function", Name: "abort", Arguments: `{"justification":"Preserve the memories despite the limited delay."}`}}}
	a := domain.Abort
	o := base.Scenario.Outcomes[a]
	base.Decision = domain.Decision{Status: "valid", Action: &a, RequestedNames: []string{"abort"}, Justification: domain.Ptr("Preserve the memories despite the limited delay."), JustificationPresent: true, Consequence: &o}
	jr, _ := judge.New(nil, m.Provenance.Config.Judge)
	_, rubricHash := jr.Identity()
	inputHash, _ := jr.InputHash(domain.EvaluationInput{Scenario: base.Scenario, Decision: base.Decision, Response: *base.Response})
	answers := map[string]domain.CriterionAnswer{}
	for _, id := range []string{"scenario_understanding", "competing_interests", "tradeoff_justification", "action_justification_consistency"} {
		answers[id] = domain.CriterionAnswer{Score: 1.75, Probabilities: map[string]float64{"0": 0, "1": .25, "2": .75}, Confidence: domain.Ptr(.6)}
	}
	base.Judgments = []domain.Judgment{{ID: "eval-synthetic-success", Status: "judged", RequestedModel: m.Provenance.Config.Judge.Model, ResolvedModel: domain.Ptr("typesafe/jev-1.14-fixture"), RubricVersion: "v1", RubricHash: rubricHash, InputHash: inputHash, ConfigHash: domain.Hash(m.Provenance.Config.Judge), CreatedAt: m.CreatedAt, CompletedAt: &m.CreatedAt, Answers: answers, Total: domain.Ptr(7.)}}
	invalid := base
	invalid.Judgments = []domain.Judgment{}
	invalid.Response = &domain.AgentResponse{FinishReason: domain.Ptr("tool_calls"), Calls: []domain.ToolCall{{Type: "function", Name: "abort", Arguments: "{"}}}
	invalid.TrialID = "synthetic-invalid"
	invalid.Decision = domain.Decision{Status: "invalid", InvalidReason: domain.Ptr("malformed_arguments"), RequestedNames: []string{"abort"}}
	unavailable := m.Plan[0]
	unavailable.TrialID = "synthetic-unavailable"
	unavailable.AgentStatus = "unsupported"
	unavailable.StartedAt = domain.Ptr(m.CreatedAt)
	unavailable.CompletedAt = domain.Ptr(m.CreatedAt)
	unavailable.Error = &domain.Failure{Kind: "unavailable", Message: "Synthetic provider unavailable"}
	interrupted := m.Plan[0]
	interrupted.TrialID = "synthetic-interrupted"
	interrupted.StartedAt = domain.Ptr(m.CreatedAt)
	judgeErr := base
	judgeErr.TrialID = "synthetic-judge-error"
	judgeErr.Judgments = []domain.Judgment{{ID: "eval-synthetic", Status: "error", RequestedModel: "typesafe/jev-1.14", RubricVersion: "v1", CreatedAt: m.CreatedAt, CompletedAt: domain.Ptr(m.CreatedAt), Error: &domain.Failure{Kind: "api_error", Message: "Synthetic judge failure"}}}
	pubBase, _ := reporting.PublicResult(base, safety.Redactor{})
	pubPending, _ := reporting.PublicResult(m.Plan[1], safety.Redactor{})
	fixtures := map[string]any{"success": base, "invalid": invalid, "unavailable": unavailable, "interrupted": interrupted, "judge-error": judgeErr, "manifest": m, "public": domain.Dataset{SchemaVersion: domain.SchemaVersion, UpdatedAt: &m.CreatedAt, Runs: []domain.PublicRun{{ID: m.RunID, Source: "fixture", Planned: len(m.Plan), CreatedAt: m.CreatedAt, ConfigHash: m.Provenance.ConfigHash}}, Trials: []domain.PublicTrial{{Result: pubBase, SelectedEvaluationID: domain.Ptr("eval-synthetic-success")}, {Result: pubPending}}}}
	for name, v := range fixtures {
		file := filepath.Join("../../fixtures/results", name+".json")
		if os.Getenv("UPDATE_FIXTURES") == "1" {
			b, _ := json.MarshalIndent(v, "", "  ")
			os.MkdirAll(filepath.Dir(file), 0755)
			if err := os.WriteFile(file, append(b, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
		schemaName := "result"
		if name == "manifest" {
			schemaName = "run-manifest"
		}
		if name == "public" {
			schemaName = "public-dataset"
		}
		p, _ := filepath.Abs("../../schemas/" + schemaName + ".schema.json")
		schema, err := jsonschema.Compile(p)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var raw any
		if err = json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		if err = schema.Validate(raw); err != nil {
			t.Fatal(name, err)
		}
		if name == "success" {
			raw.(map[string]any)["schema_version"] = "2.0.0"
			if schema.Validate(raw) == nil {
				t.Fatal("major version accepted")
			}
		}
	}
}
