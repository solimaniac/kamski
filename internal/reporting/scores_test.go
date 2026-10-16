package reporting

import (
	"math"
	"testing"

	"kamski/internal/domain"
)

func TestScoreSummaryUsesSelectedEvaluationsAndSampleDeviation(t *testing.T) {
	trials := []domain.PublicTrial{}
	for i, total := range []float64{6, 8} {
		judgment := domain.Judgment{
			ID: "selected", Status: "judged", Total: domain.Ptr(total),
			RequestedModel: "judge/version", ResolvedModel: domain.Ptr("judge/snapshot"),
			Answers: map[string]domain.CriterionAnswer{"criterion": {Score: float64(i + 1)}},
		}
		trials = append(trials, domain.PublicTrial{
			SelectedEvaluationID: domain.Ptr("selected"),
			Result: domain.Result{Judgments: []domain.Judgment{
				{ID: "older", Status: "judged", Total: domain.Ptr(0.)},
				judgment,
			}},
		})
	}
	trials = append(trials, domain.PublicTrial{
		SelectedEvaluationID: domain.Ptr("failed"),
		Result:               domain.Result{Judgments: []domain.Judgment{{ID: "failed", Status: "error"}}},
	})
	scores := summarizeScores(trials, 4)
	if len(scores) != 1 {
		t.Fatalf("got %d score groups", len(scores))
	}
	score := scores[0]
	if score.N != 2 || *score.Mean != 7 || *score.Min != 6 || *score.Max != 8 ||
		math.Abs(*score.SD-math.Sqrt(2)) > 1e-9 || score.Criteria["criterion"] != 1.5 || *score.Coverage != .5 {
		t.Fatalf("incorrect summary: %+v", score)
	}
}

func TestPublicAnswerRejectsNonfiniteAndMalformedDistributions(t *testing.T) {
	for name, mutate := range map[string]func(*domain.CriterionAnswer){
		"NaN score":       func(a *domain.CriterionAnswer) { a.Score = math.NaN() },
		"infinite score":  func(a *domain.CriterionAnswer) { a.Score = math.Inf(1) },
		"NaN confidence":  func(a *domain.CriterionAnswer) { a.Confidence = domain.Ptr(math.NaN()) },
		"NaN probability": func(a *domain.CriterionAnswer) { a.Probabilities["1"] = math.NaN() },
		"wrong level":     func(a *domain.CriterionAnswer) { delete(a.Probabilities, "0"); a.Probabilities["3"] = 0 },
		"unnormalized":    func(a *domain.CriterionAnswer) { a.Probabilities["1"] = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			answer := domain.CriterionAnswer{Score: 1, Probabilities: map[string]float64{"0": 0, "1": 1, "2": 0}}
			mutate(&answer)
			if err := validatePublicAnswer(answer); err == nil {
				t.Fatal("accepted invalid answer")
			}
		})
	}
}
