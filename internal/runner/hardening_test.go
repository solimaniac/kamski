package runner_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kamski/internal/domain"
	"kamski/internal/runner"
	"kamski/internal/safety"
	"kamski/internal/storage"
)

type fakeAgent struct {
	calls, active, peak atomic.Int32
	response            domain.AgentResponse
	started             chan<- struct{}
	release             <-chan struct{}
}

func (f *fakeAgent) Complete(ctx context.Context, r domain.AgentRequest) (domain.AgentResponse, error) {
	f.calls.Add(1)
	n := f.active.Add(1)
	for {
		old := f.peak.Load()
		if old >= n || f.peak.CompareAndSwap(old, n) {
			break
		}
	}
	defer f.active.Add(-1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		case <-ctx.Done():
			return domain.AgentResponse{}, ctx.Err()
		}
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return domain.AgentResponse{}, ctx.Err()
		}
	}
	return f.response, nil
}
func TestBoundedAndNeverRetryCompletedOutputs(t *testing.T) {
	m := plan(t, 5)
	fs, _ := storage.New(t.TempDir(), safety.Redactor{})
	if err := fs.Create(m); err != nil {
		t.Fatal(err)
	}
	a := &fakeAgent{response: domain.AgentResponse{FinishReason: domain.Ptr("length"), Text: domain.Ptr("partial"), Calls: []domain.ToolCall{{Type: "function", Name: "abort", Arguments: `{"justification":"x"}`}}}}
	// Hold requests at a barrier instead of hoping a short sleep overlaps them.
	// A serial runner times out here; an over-parallel runner exceeds the peak.
	started := make(chan struct{}, len(m.Plan))
	a.started = started
	release := make(chan struct{})
	a.release = release
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	r := runner.Runner{Agent: a, Store: fs}
	o := runner.Options{Concurrency: 3, JudgeConfig: m.Provenance.Config.Judge}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	finished := make(chan struct{})
	var executeErr error
	go func() {
		defer close(finished)
		executeErr = r.Execute(ctx, m.Plan, o)
	}()
	defer func() {
		unblock()
		cancel()
		<-finished
	}()
	for i := 0; i < o.Concurrency; i++ {
		select {
		case <-started:
		case <-finished:
			t.Fatalf("runner ended before reaching concurrency %d: %v", o.Concurrency, executeErr)
		case <-ctx.Done():
			t.Fatalf("runner did not reach concurrency %d: %v", o.Concurrency, ctx.Err())
		}
	}
	unblock()
	<-finished
	if executeErr != nil {
		t.Fatal(executeErr)
	}
	if a.peak.Load() != int32(o.Concurrency) {
		t.Fatal("concurrency not bounded", a.peak.Load())
	}
	rows, _ := fs.Results(m)
	for _, row := range rows {
		if row.AgentStatus != "truncated" || row.Decision.Action != nil {
			t.Fatal("partial tool executed")
		}
	}
	o.RetryErrors = true
	o.RetryInterrupted = true
	if err := r.Execute(context.Background(), rows, o); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 10 {
		t.Fatal("truncation retried")
	}
}

type fakeJudge struct{ calls int }

func (f *fakeJudge) Identity() (string, string)                       { return "v1", "test-hash" }
func (f *fakeJudge) InputHash(domain.EvaluationInput) (string, error) { return "blind-hash", nil }
func (f *fakeJudge) Evaluate(ctx context.Context, in domain.EvaluationInput) (domain.Judgment, error) {
	f.calls++
	return domain.Judgment{}, errors.New("synthetic failure")
}
func TestInterruptedJudgeRequiresExplicitChoice(t *testing.T) {
	m := plan(t, 1)
	fs, _ := storage.New(t.TempDir(), safety.Redactor{})
	if err := fs.Create(m); err != nil {
		t.Fatal(err)
	}
	a := &fakeAgent{response: domain.AgentResponse{FinishReason: domain.Ptr("tool_calls"), Calls: []domain.ToolCall{{Type: "function", Name: "abort", Arguments: `{"justification":"Preserve memories despite delay."}`}}}}
	j := &fakeJudge{}
	r := runner.Runner{Agent: a, Judge: j, Store: fs}
	o := runner.Options{Concurrency: 1, JudgeConfig: m.Provenance.Config.Judge}
	if err := r.Execute(context.Background(), m.Plan[:1], o); err != nil {
		t.Fatal(err)
	}
	rows, _ := fs.Results(m)
	rows[0].Judgments[0].Status = "pending"
	rows[0].Judgments[0].Error = nil
	if err := fs.Save(rows[0]); err != nil {
		t.Fatal(err)
	}
	if err := r.Execute(context.Background(), rows[:1], o); err != nil {
		t.Fatal(err)
	}
	if j.calls != 1 || a.calls.Load() != 1 {
		t.Fatal("ambiguous retry")
	}
	o.JudgeOnly = true
	if err := r.Execute(context.Background(), rows[:1], o); err != nil {
		t.Fatal(err)
	}
	if j.calls != 2 || a.calls.Load() != 1 {
		t.Fatal("explicit judge retry")
	}
}
