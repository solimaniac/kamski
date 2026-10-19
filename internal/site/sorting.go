package site

import (
	"sort"

	"kamski/internal/domain"
	"kamski/internal/reporting"
)

// Snapshot of Artificial Analysis Intelligence Index v4.3.2, retrieved
// 2026-10-19: https://artificialanalysis.ai/evaluations/artificial-analysis-intelligence-index
// Use the highest-scoring listed configuration of the exact model, not related
// variants (e.g. Muse Wonder Full). These are display priorities only: our saved
// benchmark settings are not asserted to match the index's reasoning settings.
// Keep this local so offline builds are reproducible. Unknown models follow all
// ranked models, alphabetically; ties use model name and then group key.
var intelligenceIndex = map[string]float64{
	"anthropic/claude-opus-5.5":    57.6223698102963,
	"anthropic/claude-fable-5.1":   53.3549259623252,
	"openai/gpt-6-astra":           52.673669395513,
	"google/gemini-4-argon":        52.5605655745982,
	"meta/muse-wonder":             52.393894697293,
	"xiaomi/mimo-v2.6-pro":         46.3242065310383,
	"qwen/qwen3.8-max-0902":        45.4152084980521,
	"z-ai/glm-5.3":                 44.777392385614,
	"moonshotai/kimi-k3":           43.5938229518782,
	"anthropic/claude-haiku-5.5":   43.3950199670746,
	"google/gemini-3.8-flash":      40.9262321765904,
	"deepseek/deepseek-v4.1-flash": 39.456167472527,
	"meta/muse-spark-open":         38.8666662997805,
	"openai/gpt-6-luna":            38.1245186869738,
}

func intelligenceLess(a, b reporting.Group) bool {
	x, xKnown := intelligenceIndex[a.Model]
	y, yKnown := intelligenceIndex[b.Model]
	if xKnown != yKnown {
		return xKnown
	}
	if x != y {
		return x > y
	}
	if a.Model != b.Model {
		return a.Model < b.Model
	}
	return a.Key < b.Key
}

// scoreRows keeps every judge/rubric snapshot separate while sorting all chart
// rows by their own full-precision mean. Missing scores appear last.
func scoreRows(groups []reporting.Group) []reporting.Group {
	rows := []reporting.Group{}
	for _, g := range groups {
		if len(g.Scores) == 0 {
			rows = append(rows, g)
		}
		for _, score := range g.Scores {
			row := g
			row.Scores = []reporting.Scores{score}
			rows = append(rows, row)
		}
	}
	mean := func(g reporting.Group) *float64 {
		if len(g.Scores) == 0 {
			return nil
		}
		return g.Scores[0].Mean
	}
	sort.SliceStable(rows, func(i, j int) bool {
		x, y := mean(rows[i]), mean(rows[j])
		if (x == nil) != (y == nil) {
			return x != nil
		}
		if x != nil && y != nil && *x != *y {
			return *x > *y
		}
		if rows[i].Key == rows[j].Key && len(rows[i].Scores) > 0 && len(rows[j].Scores) > 0 {
			return rows[i].Scores[0].Key < rows[j].Scores[0].Key
		}
		return intelligenceLess(rows[i], rows[j])
	})
	return rows
}

func sortRepetitions(trials []domain.PublicTrial) {
	sort.SliceStable(trials, func(i, j int) bool {
		a, b := trials[i].Result, trials[j].Result
		if a.Repetition != b.Repetition {
			return a.Repetition < b.Repetition
		}
		if a.RunID != b.RunID {
			return a.RunID < b.RunID
		}
		return a.TrialID < b.TrialID
	})
}
