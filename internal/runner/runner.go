// Package runner orchestrates one candidate completion and independent evaluations.
package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

type AgentClient interface {
	Complete(context.Context, domain.AgentRequest) (domain.AgentResponse, error)
}
type Judge interface {
	Evaluate(context.Context, domain.EvaluationInput) (domain.Judgment, error)
	Identity() (string, string)
	InputHash(domain.EvaluationInput) (string, error)
}
type Store interface {
	Save(domain.Result) error
	Record(domain.Result, string, audit.Event) (domain.AttemptRef, error)
	Sanitize(domain.Result) (domain.Result, error)
}
type Runner struct {
	Agent    AgentClient
	Judge    Judge
	Store    Store
	Progress func(domain.Result)
	Now      func() time.Time
}
type Options struct {
	Concurrency      int
	RetryInterrupted bool
	RetryErrors      bool
	JudgeOnly        bool
	Rejudge          bool
	JudgeConfig      domain.JudgeSettings
	Preflight        map[string]*domain.Failure
	JudgePreflight   *domain.Failure
}

func (r *Runner) Execute(ctx context.Context, trials []domain.Result, o Options) error {
	if o.Concurrency < 1 || o.Concurrency > 16 {
		return fmt.Errorf("invalid concurrency")
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	workctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := 0; i < o.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				if workctx.Err() != nil {
					continue
				}
				err := r.trial(workctx, &trials[n], o)
				mu.Lock()
				if err != nil && first == nil {
					first = err
					cancel()
				}
				if r.Progress != nil {
					r.Progress(trials[n])
				}
				mu.Unlock()
			}
		}()
	}
loop:
	for i := range trials {
		select {
		case <-workctx.Done():
			break loop
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}
func (r *Runner) trial(ctx context.Context, trial *domain.Result, options Options) error {
	if !options.JudgeOnly {
		if trial.StartedAt != nil && trial.CompletedAt == nil {
			trial.AgentStatus = "interrupted"
			trial.Error = &domain.Failure{Kind: "interrupted", Message: "in-flight operation may have been processed or billed; explicit retry required"}
			if err := r.save(trial); err != nil {
				return err
			}
		}
		if shouldRunCandidate(*trial, options) {
			if err := r.runCandidate(ctx, trial, options); err != nil {
				return err
			}
		}
	}
	if trial.StartedAt == nil || trial.CompletedAt == nil {
		return nil
	}
	return r.evaluate(ctx, trial, options)
}
