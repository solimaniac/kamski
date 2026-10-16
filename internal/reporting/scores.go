package reporting

import (
	"math"
	"sort"

	"kamski/internal/domain"
)

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
func summarizeScores(trials []domain.PublicTrial, valid int) []Scores {
	byKey := map[string][]domain.Judgment{}
	for _, trial := range trials {
		if judgment := Selected(trial); judgment != nil && judgment.Status == "judged" && judgment.Total != nil {
			key := ScoringKey(*judgment)
			byKey[key] = append(byKey[key], *judgment)
		}
	}
	scores := []Scores{}
	for key, judgments := range byKey {
		scores = append(scores, summarizeScoreGroup(key, judgments, valid))
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Key < scores[j].Key })
	return scores
}

func summarizeScoreGroup(key string, judgments []domain.Judgment, valid int) Scores {
	first := judgments[0]
	score := Scores{
		Key:      key,
		Judge:    first.RequestedModel,
		Rubric:   first.RubricVersion,
		Hash:     first.RubricHash,
		N:        len(judgments),
		Criteria: map[string]float64{},
		Coverage: Ratio(len(judgments), valid),
	}
	if first.ResolvedModel != nil {
		score.Resolved = *first.ResolvedModel
	}
	sum, low, high := 0.0, *first.Total, *first.Total
	for _, judgment := range judgments {
		total := *judgment.Total
		sum += total
		low = min(low, total)
		high = max(high, total)
		for criterion, answer := range judgment.Answers {
			score.Criteria[criterion] += answer.Score / float64(len(judgments))
		}
	}
	mean := sum / float64(len(judgments))
	score.Mean = &mean
	score.Min = &low
	score.Max = &high
	if len(judgments) > 1 {
		variance := 0.0
		for _, judgment := range judgments {
			variance += math.Pow(*judgment.Total-mean, 2)
		}
		score.SD = domain.Ptr(math.Sqrt(variance / float64(len(judgments)-1)))
	}
	return score
}
