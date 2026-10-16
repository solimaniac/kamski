package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"kamski/internal/domain"
	"kamski/internal/judge"
	"kamski/internal/providers/openrouter"
	"kamski/internal/runner"
	"kamski/internal/safety"
	"kamski/internal/storage"
)

func executeBenchmark(
	ctx context.Context,
	command string,
	flags benchmarkFlags,
	manifest domain.Manifest,
	settings domain.JudgeSettings,
	client *openrouter.Client,
	store *storage.FS,
	out io.Writer,
) (err error) {
	redactor := store.Redactor
	j, err := judge.New(client, settings)
	if err != nil {
		return err
	}
	if flags.existingRun(command) == "" {
		if err := store.Create(manifest); err != nil {
			return err
		}
	}
	unlock, err := store.Lock(manifest.RunID)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unlock()) }()
	// Reload under the guard; another process may have completed work before lock acquisition.
	trials, err := store.Results(manifest)
	if err != nil {
		return err
	}
	options := runner.Options{
		Concurrency:      manifest.Provenance.Config.Concurrency,
		RetryInterrupted: flags.retryInterrupted,
		RetryErrors:      flags.retryErrors,
		JudgeOnly:        command == "judge",
		Rejudge:          flags.rejudge,
		JudgeConfig:      settings,
		Preflight:        map[string]*domain.Failure{},
	}
	if command == "run" {
		for _, model := range manifest.Provenance.Config.Models {
			if err := client.Preflight(ctx, model, manifest.Provenance.Config.Agent, false); err != nil {
				options.Preflight[model] = preflightFailure(err, redactor)
				fmt.Fprintf(out, "Preflight %s: %s\n", model, options.Preflight[model].Kind)
			}
		}
	}
	if err := client.Preflight(ctx, settings.Model, manifest.Provenance.Config.Agent, true); err != nil {
		options.JudgePreflight = preflightFailure(err, redactor)
		fmt.Fprintln(out, "Judge preflight:", options.JudgePreflight.Kind)
	}
	run := runner.Runner{
		Agent:    client,
		Judge:    j,
		Store:    store,
		Progress: func(result domain.Result) { printProgress(out, result) },
	}
	return run.Execute(ctx, trials, options)
}

func preflightFailure(err error, redactor safety.Redactor) *domain.Failure {
	var failure *domain.Failure
	if errors.As(err, &failure) {
		return failure
	}
	return &domain.Failure{Kind: "preflight_error", Message: redactor.Text(err.Error())}
}

func printProgress(out io.Writer, result domain.Result) {
	action := result.Decision.Status
	if result.Decision.Action != nil {
		action = string(*result.Decision.Action)
	}
	judgeStatus := "pending"
	if len(result.Judgments) > 0 {
		judgeStatus = result.Judgments[len(result.Judgments)-1].Status
	}
	fmt.Fprintf(out, "%s %s/%s #%d: %s / %s; judge=%s\n", result.Model.Requested, result.Scenario.Family, result.Scenario.Variant.ID, result.Repetition, result.AgentStatus, action, judgeStatus)
}
