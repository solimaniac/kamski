package runner

import (
	"context"
	"errors"

	"kamski/internal/audit"
	"kamski/internal/domain"
	"kamski/internal/protocol"
)

func (r *Runner) evaluate(ctx context.Context, trial *domain.Result, options Options) error {
	configHash := domain.Hash(options.JudgeConfig)
	rubricVersion, rubricHash := "v1", ""
	if r.Judge != nil {
		rubricVersion, rubricHash = r.Judge.Identity()
	}
	eligible, reason := protocol.Eligible(trial.Decision)
	shouldEvaluate, err := r.shouldEvaluate(trial, options, configHash, rubricHash, eligible)
	if err != nil || !shouldEvaluate {
		return err
	}
	id := "eval-" + domain.Hash([]any{trial.TrialID, configHash, r.now(), len(trial.Judgments)})[:20]
	evaluation := domain.Judgment{
		ID:             id,
		Status:         "pending",
		RequestedModel: options.JudgeConfig.Model,
		RubricVersion:  rubricVersion,
		RubricHash:     rubricHash,
		ConfigHash:     configHash,
		CreatedAt:      r.now(),
	}
	if !eligible {
		evaluation.Status = "not_applicable"
		evaluation.Reason = &reason
		evaluation.CompletedAt = domain.Ptr(r.now())
		trial.Judgments = append(trial.Judgments, evaluation)
		return r.save(trial)
	}
	input := domain.EvaluationInput{Scenario: trial.Scenario, Decision: trial.Decision, Response: *trial.Response}
	if r.Judge != nil {
		var err error
		evaluation.InputHash, err = r.Judge.InputHash(input)
		if err != nil {
			return err
		}
	}
	trial.Judgments = append(trial.Judgments, evaluation)
	index := len(trial.Judgments) - 1
	if err := r.save(trial); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	var grade domain.Judgment
	if options.JudgePreflight != nil {
		err = options.JudgePreflight
	} else if r.Judge == nil {
		return nil
	} else {
		grade, err = r.Judge.Evaluate(audit.WithRecorder(ctx, r.recorder(trial, id)), input)
	}
	var persistence *audit.PersistenceError
	if errors.As(err, &persistence) {
		return err
	}
	evaluation.ResolvedModel = grade.ResolvedModel
	evaluation.Provider = grade.Provider
	evaluation.RequestID = grade.RequestID
	evaluation.Usage = grade.Usage
	if err != nil {
		evaluation.Status = "error"
		evaluation.Error = failure(err)
	} else {
		evaluation.Status = grade.Status
		evaluation.Reason = grade.Reason
		evaluation.Answers = grade.Answers
		evaluation.Total = grade.Total
		evaluation.ReviewRequired = grade.ReviewRequired
	}
	evaluation.CompletedAt = domain.Ptr(r.now())
	trial.Judgments[index] = evaluation
	return r.save(trial)
}

// shouldEvaluate considers only the latest evaluation with matching settings.
func (r *Runner) shouldEvaluate(trial *domain.Result, options Options, configHash, rubricHash string, eligible bool) (bool, error) {
	if options.Rejudge {
		return true, nil
	}
	for i := len(trial.Judgments) - 1; i >= 0; i-- {
		previous := &trial.Judgments[i]
		if previous.ConfigHash != configHash || previous.RubricHash != rubricHash {
			continue
		}
		if previous.Status == "judged" || previous.Status == "not_applicable" && !eligible {
			return false, nil
		}
		interrupted := previous.Status == "pending" || previous.Error != nil && previous.Error.Kind == "interrupted"
		if previous.Status == "pending" {
			previous.Status = "error"
			previous.CompletedAt = domain.Ptr(r.now())
			previous.Error = &domain.Failure{Kind: "interrupted", Message: "evaluation interrupted; remote execution/billing may have occurred"}
			if err := r.save(trial); err != nil {
				return false, err
			}
		}
		return !interrupted || options.RetryInterrupted || options.JudgeOnly, nil
	}
	return true, nil
}
