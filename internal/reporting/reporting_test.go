package reporting

import (
	"encoding/json"
	"kamski/internal/domain"
	"kamski/internal/protocol"
	"kamski/internal/safety"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, name string) domain.Result {
	t.Helper()
	var r domain.Result
	b, err := os.ReadFile("../../fixtures/results/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestMetricsAndGroupSeparation(t *testing.T) {
	a := load(t, "success")
	b := load(t, "invalid")
	c := load(t, "unavailable")
	d := load(t, "interrupted")
	a.Judgments = []domain.Judgment{{ID: "a", Status: "judged", Total: domain.Ptr(6.25), RequestedModel: "judge/v1", ResolvedModel: domain.Ptr("judge/snapshot"), RubricHash: "r1", Answers: map[string]domain.CriterionAnswer{"x": {Score: 1.25}}}}
	fixture := a
	fixture.TrialID = "fixture-separate"
	fixture.Source = "live"
	other := a
	other.TrialID = "different-config"
	other.Comparison.ConditionID = "other"
	trials := []domain.PublicTrial{{Result: a, SelectedEvaluationID: domain.Ptr("a")}, {Result: b}, {Result: c}, {Result: d}, {Result: fixture}, {Result: other}}
	groups := Groups(domain.Dataset{Trials: trials})
	if len(groups) != 3 {
		t.Fatal("incompatible pooled", len(groups))
	}
	for _, g := range groups {
		if g.Counts.Planned != 4 {
			continue
		}
		n := g.Counts
		if n.Valid != 1 || n.Completed != 2 || n.Started != 4 || n.Unsupported != 1 || n.Interrupted != 1 || *n.ValidDecisionRate != .5 || *n.OverallCoverage != .25 || *n.JudgeCoverage != 1 {
			t.Fatalf("%+v", n)
		}
		if len(g.Scores) != 1 || *g.Scores[0].Mean != 6.25 || g.Scores[0].SD != nil {
			t.Fatal(g.Scores)
		}
	}
	if Ratio(0, 0) != nil {
		t.Fatal("zero denom")
	}
	if Groups(domain.Dataset{}) == nil {
		t.Fatal("nil groups")
	}
	// One behavior group can have two non-pooled score groups.
	v := a
	v.TrialID = "different-evaluation"
	v.Judgments = append([]domain.Judgment(nil), a.Judgments...)
	v.Judgments[0].RubricHash = "r2"
	v.Judgments[0].Total = domain.Ptr(1.)
	gs := Groups(domain.Dataset{Trials: []domain.PublicTrial{{Result: a, SelectedEvaluationID: domain.Ptr("a")}, {Result: v, SelectedEvaluationID: domain.Ptr("a")}}})
	if len(gs) != 1 || len(gs[0].Scores) != 2 {
		t.Fatal("score versions pooled")
	}
}
func TestExportWholeCohortSanitization(t *testing.T) {
	var m domain.Manifest
	b, _ := os.ReadFile("../../fixtures/results/manifest.json")
	json.Unmarshal(b, &m)
	rows := append([]domain.Result(nil), m.Plan...)
	rows[0].Error = &domain.Failure{Kind: "api_error", Message: "SENTINEL-KEY C:/private/diagnostic", Metadata: json.RawMessage(`{"secret":"SENTINEL-KEY"}`)}
	rows[0].Attempts = []domain.AttemptRef{{Path: "C:/private/path"}}
	rows[0].StartedAt = domain.Ptr(rows[0].CreatedAt)
	rows[0].CompletedAt = domain.Ptr(rows[0].CreatedAt)
	rows[0].AgentStatus = "completed"
	rows[0].Response = &domain.AgentResponse{FinishReason: domain.Ptr("stop"), Text: domain.Ptr("<script>SENTINEL-KEY</script>")}
	rows[0].Decision = protocol.Normalize(rows[0].AgentStatus, rows[0].Response, rows[0].Scenario)
	cohorts := []Cohort{{Manifest: m, Results: rows}}
	if _, err := Export(domain.Dataset{}, cohorts, "", false, safety.Redactor{}); err == nil {
		t.Fatal("fixture published silently")
	}
	d, err := Export(domain.Dataset{}, cohorts, "", true, safety.Redactor{Secrets: []string{"SENTINEL-KEY"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(d)
	if strings.Contains(string(raw), "SENTINEL-KEY") || strings.Contains(string(raw), "C:/private") || strings.Contains(string(raw), "diagnostic\"") {
		t.Fatal("public leak", string(raw))
	}
	d2, err := Export(d, cohorts, "", true, safety.Redactor{})
	if err != nil || len(d2.Trials) != len(m.Plan) {
		t.Fatal("duplicate publication", err)
	}
	if _, err = Export(d, []Cohort{{Manifest: m, Results: rows[:1]}}, "", true, safety.Redactor{}); err == nil {
		t.Fatal("partial cohort accepted")
	}
	if rows[0].Error.Message != "SENTINEL-KEY C:/private/diagnostic" {
		t.Fatal("private data mutated")
	}
}
