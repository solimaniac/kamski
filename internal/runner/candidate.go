package runner

import (
	"context"
	"errors"
	"fmt"

	"kamski/internal/audit"
	"kamski/internal/domain"
	"kamski/internal/protocol"
)

func shouldRunCandidate(trial domain.Result, options Options) bool {
	if trial.CompletedAt == nil {
		return trial.StartedAt == nil || options.RetryInterrupted
	}
	if trial.Response != nil {
		return false
	}
	switch trial.AgentStatus {
	case "api_error", "unsupported", "provider_blocked", "protocol_error":
		return options.RetryErrors
	default:
		return false
	}
}

func (r *Runner) runCandidate(ctx context.Context, trial *domain.Result, options Options) error {
	trial.StartedAt = domain.Ptr(r.now())
	trial.CompletedAt = nil
	trial.Response = nil
	trial.Error = nil
	trial.AgentStatus = "interrupted"
	trial.Decision = domain.Decision{Status: "not_evaluated", RequestedNames: []string{}}
	if err := r.save(trial); err != nil {
		return err
	}

	var response domain.AgentResponse
	var err error
	if preflight := options.Preflight[trial.Model.Requested]; preflight != nil {
		err = preflight
	} else {
		if r.Agent == nil {
			return fmt.Errorf("candidate client missing")
		}
		request := domain.AgentRequest{
			Model:    trial.Model.Requested,
			Messages: trial.Scenario.Messages,
			Tools:    trial.Scenario.Tools,
			Settings: trial.Comparison.Settings,
		}
		response, err = r.Agent.Complete(audit.WithRecorder(ctx, r.recorder(trial, "")), request)
	}
	var persistence *audit.PersistenceError
	if errors.As(err, &persistence) {
		return err
	}
	if err != nil {
		trial.Error = failure(err)
		trial.AgentStatus = status(trial.Error)
	} else {
		trial.Response = &response
		trial.Model.Resolved = response.Model
		trial.Model.Provider = response.Provider
		trial.AgentStatus = protocol.AgentStatus(response)
	}
	if trial.AgentStatus != "interrupted" {
		trial.CompletedAt = domain.Ptr(r.now())
	}
	trial.Decision = protocol.Normalize(trial.AgentStatus, trial.Response, trial.Scenario)
	return r.save(trial)
}
