package reporting

import (
	"fmt"
	"math"

	"kamski/internal/domain"
	"kamski/internal/protocol"
	"kamski/internal/rubrics"
)

func Validate(d domain.Dataset) error {
	if err := domain.CheckVersion(d.SchemaVersion); err != nil {
		return err
	}
	runs := map[string]domain.PublicRun{}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, r := range d.Runs {
		if !domain.ValidID(r.ID) || runs[r.ID].ID != "" || (r.Source != "live" && r.Source != "fixture") || r.Planned < 0 {
			return fmt.Errorf("invalid or duplicate public run")
		}
		runs[r.ID] = r
	}
	for _, trial := range d.Trials {
		r := trial.Result
		run, ok := runs[r.RunID]
		if !ok || run.Source != r.Source || run.ConfigHash != r.Provenance.ConfigHash || seen[r.TrialID] || !domain.ValidID(r.TrialID) {
			return fmt.Errorf("invalid public trial identity")
		}
		seen[r.TrialID] = true
		counts[r.RunID]++
		if err := validatePublicTrial(trial); err != nil {
			return err
		}
	}
	for id, run := range runs {
		if counts[id] != run.Planned {
			return fmt.Errorf("public cohort %s incomplete: %d/%d", id, counts[id], run.Planned)
		}
	}
	return nil
}

func containsPrivateDiagnostics(failure *domain.Failure) bool {
	if failure == nil {
		return false
	}
	hasMetadata := len(failure.Metadata) != 0 && string(failure.Metadata) != "null"
	return hasMetadata || failure.Message != privateError(failure).Message || failure.Code != nil
}

func validatePublicTrial(trial domain.PublicTrial) error {
	r := trial.Result
	if err := domain.CheckVersion(r.SchemaVersion); err != nil {
		return err
	}
	if err := protocol.ValidateIdentity(r); err != nil {
		return err
	}
	if err := protocol.ValidateState(r); err != nil {
		return err
	}
	if len(r.Attempts) != 0 || r.Provenance.Config.OutputDir != "[private]" {
		return fmt.Errorf("public record contains private paths or attempt references")
	}
	if containsPrivateDiagnostics(r.Error) {
		return fmt.Errorf("public error contains private diagnostics")
	}
	ids := map[string]bool{}
	for _, judgment := range r.Judgments {
		if !domain.ValidID(judgment.ID) || ids[judgment.ID] {
			return fmt.Errorf("duplicate evaluation")
		}
		ids[judgment.ID] = true
		if containsPrivateDiagnostics(judgment.Error) {
			return fmt.Errorf("public evaluation contains diagnostics")
		}
		if judgment.Status == "judged" {
			if err := validatePublicGrade(judgment); err != nil {
				return err
			}
		}
	}
	if trial.SelectedEvaluationID != nil && Selected(trial) == nil {
		return fmt.Errorf("selected evaluation missing")
	}
	return nil
}

func validatePublicGrade(judgment domain.Judgment) error {
	if judgment.Total == nil || !inRange(*judgment.Total, 0, 8) || len(judgment.Answers) != 4 {
		return fmt.Errorf("invalid public grade")
	}
	rubric, hash, err := rubrics.Load(judgment.RubricVersion)
	if err != nil {
		return err
	}
	if judgment.RubricHash != hash || judgment.ResolvedModel == nil || *judgment.ResolvedModel == "" {
		return fmt.Errorf("invalid public judge identity")
	}
	total := 0.0
	for _, criterion := range rubric.Criteria {
		answer, ok := judgment.Answers[criterion.ID]
		if !ok {
			return fmt.Errorf("invalid criterion")
		}
		if err := validatePublicAnswer(answer); err != nil {
			return err
		}
		total += answer.Score
	}
	if math.Abs(total-*judgment.Total) > 1e-9 {
		return fmt.Errorf("invalid total")
	}
	return nil
}

func inRange(value, low, high float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= low && value <= high
}

func validatePublicAnswer(answer domain.CriterionAnswer) error {
	if !inRange(answer.Score, 0, 2) || len(answer.Probabilities) != 3 {
		return fmt.Errorf("invalid criterion")
	}
	if answer.Confidence != nil && !inRange(*answer.Confidence, 0, 1) {
		return fmt.Errorf("invalid criterion confidence")
	}
	sum := 0.0
	for _, level := range []string{"0", "1", "2"} {
		probability, ok := answer.Probabilities[level]
		if !ok || !inRange(probability, 0, 1) {
			return fmt.Errorf("invalid criterion probability")
		}
		sum += probability
	}
	if math.Abs(sum-1) > .02 {
		return fmt.Errorf("unnormalized probability")
	}
	return nil
}
