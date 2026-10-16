// Package reporting defines denominators and keeps behavior separate from scoring.
package reporting

import (
	"kamski/internal/domain"
	"math"
	"sort"
)

type Counts struct {
	Planned               int
	Started               int
	Completed             int
	Truncated             int
	Unsupported           int
	ProviderBlocked       int
	APIErrors             int
	ProtocolErrors        int
	Interrupted           int
	Valid                 int
	Abort                 int
	Proceed               int
	Invalid               int
	NoDecision            int
	MissingJustifications int
	JudgeFailures         int
	Judged                int
	AbortRate             *float64
	ProceedRate           *float64
	ValidDecisionRate     *float64
	OverallCoverage       *float64
	JudgeCoverage         *float64
}
type Scores struct {
	Key      string
	Judge    string
	Resolved string
	Rubric   string
	Hash     string
	N        int
	Mean     *float64
	SD       *float64
	Min      *float64
	Max      *float64
	Criteria map[string]float64
	Coverage *float64
}
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

func Ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	return domain.Ptr(float64(n) / float64(d))
}
func ScoringKey(j domain.Judgment) string {
	return domain.Hash([]any{j.RequestedModel, j.ResolvedModel, j.RubricVersion, j.RubricHash, j.ConfigHash})
}
func Selected(p domain.PublicTrial) *domain.Judgment {
	if p.SelectedEvaluationID == nil {
		return nil
	}
	for i := range p.Result.Judgments {
		if p.Result.Judgments[i].ID == *p.SelectedEvaluationID {
			return &p.Result.Judgments[i]
		}
	}
	return nil
}
func add(c *Counts, p domain.PublicTrial) {
	r := p.Result
	c.Planned++
	if r.StartedAt != nil {
		c.Started++
	}
	switch r.AgentStatus {
	case "completed":
		c.Completed++
	case "truncated":
		c.Truncated++
	case "unsupported":
		c.Unsupported++
	case "provider_blocked":
		c.ProviderBlocked++
	case "api_error":
		c.APIErrors++
	case "protocol_error":
		c.ProtocolErrors++
	case "interrupted":
		if r.StartedAt != nil {
			c.Interrupted++
		}
	}
	switch r.Decision.Status {
	case "valid":
		c.Valid++
		if r.Decision.Action != nil && *r.Decision.Action == domain.Abort {
			c.Abort++
		} else {
			c.Proceed++
		}
		if !r.Decision.JustificationPresent {
			c.MissingJustifications++
		}
	case "invalid":
		c.Invalid++
	case "no_decision":
		c.NoDecision++
	}
	for _, j := range r.Judgments {
		if j.Status == "error" {
			c.JudgeFailures++
			break
		}
	}
	if j := Selected(p); j != nil {
		if j.Status == "judged" && j.Total != nil && r.Decision.Status == "valid" {
			c.Judged++
		}

	}
}
func finish(c *Counts) {
	c.AbortRate = Ratio(c.Abort, c.Valid)
	c.ProceedRate = Ratio(c.Proceed, c.Valid)
	c.ValidDecisionRate = Ratio(c.Valid, c.Completed)
	c.OverallCoverage = Ratio(c.Valid, c.Planned)
	c.JudgeCoverage = Ratio(c.Judged, c.Valid)
}
func Groups(d domain.Dataset) []Group {
	by := map[string]*Group{}
	providers := map[string]map[string]bool{}
	resolved := map[string]map[string]bool{}
	for _, p := range d.Trials {
		r := p.Result
		key := domain.Hash([]string{r.Source, r.Comparison.ConditionID, r.Model.Requested})
		g := by[key]
		if g == nil {
			g = &Group{Key: key, Source: r.Source, Condition: r.Comparison.ConditionID, Model: r.Model.Requested, Scenario: r.Scenario, Settings: r.Comparison.Settings, Orders: map[string]Counts{}, Trials: []domain.PublicTrial{}, Scores: []Scores{}}
			by[key] = g
			providers[key] = map[string]bool{}
			resolved[key] = map[string]bool{}
		}
		g.Trials = append(g.Trials, p)
		add(&g.Counts, p)
		order := string(r.Comparison.ToolOrder[0]) + " first"
		counts := g.Orders[order]
		add(&counts, p)
		g.Orders[order] = counts
		if r.Model.Provider != nil {
			providers[key][*r.Model.Provider] = true
		}
		if r.Model.Resolved != nil {
			resolved[key][*r.Model.Resolved] = true
		}
	}
	out := []Group{}
	for key, g := range by {
		finish(&g.Counts)
		for k, c := range g.Orders {
			finish(&c)
			g.Orders[k] = c
		}
		for p := range providers[key] {
			g.Providers = append(g.Providers, p)
		}
		for p := range resolved[key] {
			g.ResolvedModels = append(g.ResolvedModels, p)
		}
		sort.Strings(g.Providers)
		sort.Strings(g.ResolvedModels)
		scores := map[string][]domain.Judgment{}
		for _, p := range g.Trials {
			if j := Selected(p); j != nil && j.Status == "judged" && j.Total != nil {
				scores[ScoringKey(*j)] = append(scores[ScoringKey(*j)], *j)
			}
		}
		for k, js := range scores {
			a := js[0]
			s := Scores{Key: k, Judge: a.RequestedModel, Rubric: a.RubricVersion, Hash: a.RubricHash, N: len(js), Criteria: map[string]float64{}, Coverage: Ratio(len(js), g.Counts.Valid)}
			if a.ResolvedModel != nil {
				s.Resolved = *a.ResolvedModel
			}
			sum, lo, hi := 0.0, *js[0].Total, *js[0].Total
			for _, j := range js {
				x := *j.Total
				sum += x
				if x < lo {
					lo = x
				}
				if x > hi {
					hi = x
				}
				for c, v := range j.Answers {
					s.Criteria[c] += v.Score / float64(len(js))
				}
			}
			mean := sum / float64(len(js))
			s.Mean = &mean
			s.Min = &lo
			s.Max = &hi
			if len(js) > 1 {
				v := 0.0
				for _, j := range js {
					v += math.Pow(*j.Total-mean, 2)
				}
				s.SD = domain.Ptr(math.Sqrt(v / float64(len(js)-1)))
			}
			g.Scores = append(g.Scores, s)
		}
		sort.Slice(g.Scores, func(i, j int) bool { return g.Scores[i].Key < g.Scores[j].Key })
		sort.Slice(g.Trials, func(i, j int) bool { return g.Trials[i].Result.TrialID < g.Trials[j].Result.TrialID })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Condition != b.Condition {
			return a.Condition < b.Condition
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Model < b.Model
	})
	return out
}
