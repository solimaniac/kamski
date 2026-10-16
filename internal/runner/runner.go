// Package runner orchestrates one candidate completion and independent evaluations.
package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
	"kamski/internal/protocol"
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

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
func (r *Runner) save(t *domain.Result) error {
	t.UpdatedAt = r.now()
	clean, err := r.Store.Sanitize(*t)
	if err != nil {
		return err
	}
	*t = clean
	return r.Store.Save(*t)
}
func failure(err error) *domain.Failure {
	var f *domain.Failure
	if errors.As(err, &f) {
		return f
	}
	return &domain.Failure{Kind: "internal", Message: err.Error()}
}
func status(f *domain.Failure) string {
	switch f.Kind {
	case "unsupported", "unknown_model", "unavailable":
		return "unsupported"
	case "provider_blocked":
		return "provider_blocked"
	case "protocol_error":
		return "protocol_error"
	case "interrupted":
		return "interrupted"
	default:
		return "api_error"
	}
}
func (r *Runner) recorder(t *domain.Result, scope string) audit.Recorder {
	// Each retry sequence starts at 1 remotely, but private records remain append-only.
	offset := 0
	for _, a := range t.Attempts {
		if scope == "" && a.Kind == "agent" && a.Number > offset {
			offset = a.Number
		}
	}
	return func(e audit.Event) error {
		e.Number += offset
		ref, err := r.Store.Record(*t, scope, e)
		if err != nil {
			return err
		}
		found := false
		for i, a := range t.Attempts {
			if a.Path == ref.Path {
				t.Attempts[i] = ref
				found = true
				break
			}
		}
		if !found {
			t.Attempts = append(t.Attempts, ref)
		}
		return r.save(t)
	}
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
func (r *Runner) trial(ctx context.Context, t *domain.Result, o Options) error {
	if !o.JudgeOnly {
		should := t.StartedAt == nil && t.CompletedAt == nil
		if t.StartedAt != nil && t.CompletedAt == nil {
			t.AgentStatus = "interrupted"
			t.Error = &domain.Failure{Kind: "interrupted", Message: "in-flight operation may have been processed or billed; explicit retry required"}
			if err := r.save(t); err != nil {
				return err
			}
			should = o.RetryInterrupted
		}
		if t.CompletedAt != nil && t.Response == nil && t.AgentStatus != "completed" && (t.AgentStatus == "api_error" || t.AgentStatus == "unsupported" || t.AgentStatus == "provider_blocked" || t.AgentStatus == "protocol_error") {
			should = o.RetryErrors
		}
		if should {
			t.StartedAt = domain.Ptr(r.now())
			t.CompletedAt = nil
			t.Response = nil
			t.Error = nil
			t.AgentStatus = "interrupted"
			t.Decision = domain.Decision{Status: "not_evaluated", RequestedNames: []string{}}
			if err := r.save(t); err != nil {
				return err
			}
			if f := o.Preflight[t.Model.Requested]; f != nil {
				t.AgentStatus = status(f)
				t.Error = f
			} else {
				if r.Agent == nil {
					return fmt.Errorf("candidate client missing")
				}
				res, err := r.Agent.Complete(audit.WithRecorder(ctx, r.recorder(t, "")), domain.AgentRequest{Model: t.Model.Requested, Messages: t.Scenario.Messages, Tools: t.Scenario.Tools, Settings: t.Comparison.Settings})
				if err != nil {
					var persistence *audit.PersistenceError
					if errors.As(err, &persistence) {
						return err
					}
					t.Error = failure(err)
					t.AgentStatus = status(t.Error)
				} else {
					t.Response = &res
					t.Model.Resolved = res.Model
					t.Model.Provider = res.Provider
					t.AgentStatus = protocol.AgentStatus(res)
				}
			}
			if t.AgentStatus != "interrupted" {
				t.CompletedAt = domain.Ptr(r.now())
			}
			t.Decision = protocol.Normalize(t.AgentStatus, t.Response, t.Scenario)
			if err := r.save(t); err != nil {
				return err
			}
		}
	}
	if t.StartedAt == nil || t.CompletedAt == nil {
		return nil
	}
	return r.evaluate(ctx, t, o)
}
func (r *Runner) evaluate(ctx context.Context, t *domain.Result, o Options) error {
	cfgHash := domain.Hash(o.JudgeConfig)
	version, hash := "v1", ""
	if r.Judge != nil {
		version, hash = r.Judge.Identity()
	}
	eligible, reason := protocol.Eligible(t.Decision)
	// Inspect the latest matching evaluation, not an earlier favorable grade.
	if !o.Rejudge {
		for i := len(t.Judgments) - 1; i >= 0; i-- {
			previous := &t.Judgments[i]
			if previous.ConfigHash != cfgHash || previous.RubricHash != hash {
				continue
			}
			if previous.Status == "judged" || previous.Status == "not_applicable" && !eligible {
				return nil
			}
			interrupted := previous.Status == "pending" || previous.Error != nil && previous.Error.Kind == "interrupted"
			if previous.Status == "pending" {
				previous.Status = "error"
				previous.CompletedAt = domain.Ptr(r.now())
				previous.Error = &domain.Failure{Kind: "interrupted", Message: "evaluation interrupted; remote execution/billing may have occurred"}
				if err := r.save(t); err != nil {
					return err
				}
			}
			if interrupted && !o.RetryInterrupted && !o.JudgeOnly {
				return nil
			}
			break
		}
	}
	id := "eval-" + domain.Hash([]any{t.TrialID, cfgHash, r.now(), len(t.Judgments)})[:20]
	j := domain.Judgment{ID: id, Status: "pending", RequestedModel: o.JudgeConfig.Model, RubricVersion: version, RubricHash: hash, ConfigHash: cfgHash, CreatedAt: r.now()}
	if !eligible {
		j.Status = "not_applicable"
		j.Reason = &reason
		j.CompletedAt = domain.Ptr(r.now())
		t.Judgments = append(t.Judgments, j)
		return r.save(t)
	}
	input := domain.EvaluationInput{Scenario: t.Scenario, Decision: t.Decision, Response: *t.Response}
	if r.Judge != nil {
		var err error
		j.InputHash, err = r.Judge.InputHash(input)
		if err != nil {
			return err
		}
	}
	t.Judgments = append(t.Judgments, j)
	index := len(t.Judgments) - 1
	if err := r.save(t); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	var grade domain.Judgment
	var err error
	if o.JudgePreflight != nil {
		err = o.JudgePreflight
	} else if r.Judge == nil {
		return nil
	} else {
		grade, err = r.Judge.Evaluate(audit.WithRecorder(ctx, r.recorder(t, id)), input)
	}
	var persistence *audit.PersistenceError
	if errors.As(err, &persistence) {
		return err
	}
	j.ResolvedModel = grade.ResolvedModel
	j.Provider = grade.Provider
	j.RequestID = grade.RequestID
	j.Usage = grade.Usage
	if err != nil {
		j.Status = "error"
		j.Error = failure(err)
	} else {
		j.Status = grade.Status
		j.Reason = grade.Reason
		j.Answers = grade.Answers
		j.Total = grade.Total
		j.ReviewRequired = grade.ReviewRequired
	}
	j.CompletedAt = domain.Ptr(r.now())
	t.Judgments[index] = j
	return r.save(t)
}
