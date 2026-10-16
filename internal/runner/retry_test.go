package runner

import (
	"testing"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

func TestCandidateRetryPolicy(t *testing.T) {
	now := time.Now()
	for _, tt := range []struct {
		name    string
		trial   domain.Result
		options Options
		want    bool
	}{
		{name: "unstarted", want: true},
		{name: "interrupted", trial: domain.Result{StartedAt: &now}},
		{name: "explicit interrupted retry", trial: domain.Result{StartedAt: &now}, options: Options{RetryInterrupted: true}, want: true},
		{name: "error retry does not retry interruption", trial: domain.Result{StartedAt: &now}, options: Options{RetryErrors: true}},
		{name: "completed without response", trial: domain.Result{CompletedAt: &now, AgentStatus: "completed"}, options: Options{RetryErrors: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRunCandidate(tt.trial, tt.options); got != tt.want {
				t.Fatalf("shouldRunCandidate = %v, want %v", got, tt.want)
			}
		})
	}
	for _, status := range []string{"api_error", "unsupported", "provider_blocked", "protocol_error"} {
		trial := domain.Result{StartedAt: &now, CompletedAt: &now, AgentStatus: status}
		if shouldRunCandidate(trial, Options{}) || !shouldRunCandidate(trial, Options{RetryErrors: true}) {
			t.Errorf("incorrect opt-in policy for %s", status)
		}
		trial.Response = &domain.AgentResponse{}
		if shouldRunCandidate(trial, Options{RetryErrors: true, RetryInterrupted: true}) {
			t.Errorf("retried an observed response with status %s", status)
		}
	}
}

type retryStore struct{ saves int }

func (s *retryStore) Save(domain.Result) error                      { s.saves++; return nil }
func (*retryStore) Sanitize(r domain.Result) (domain.Result, error) { return r, nil }
func (*retryStore) Record(domain.Result, string, audit.Event) (domain.AttemptRef, error) {
	return domain.AttemptRef{}, nil
}

func TestEvaluationRetryUsesLatestMatchingJudgment(t *testing.T) {
	for _, tt := range []struct {
		name      string
		latest    string
		options   Options
		want      bool
		wantSaves int
	}{
		{name: "judged", latest: "judged"},
		{name: "latest failure supersedes success", latest: "error", want: true},
		{name: "pending requires consent", latest: "pending", wantSaves: 1},
		{name: "retry interrupted", latest: "pending", options: Options{RetryInterrupted: true}, want: true, wantSaves: 1},
		{name: "judge command is explicit consent", latest: "pending", options: Options{JudgeOnly: true}, want: true, wantSaves: 1},
		{name: "rejudge appends without rewriting", latest: "pending", options: Options{Rejudge: true}, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			trial := domain.Result{Judgments: []domain.Judgment{
				{Status: "judged", ConfigHash: "config", RubricHash: "rubric"},
				{Status: tt.latest, ConfigHash: "config", RubricHash: "rubric"},
				{Status: "judged", ConfigHash: "other", RubricHash: "rubric"},
			}}
			store := &retryStore{}
			r := Runner{Store: store}
			got, err := r.shouldEvaluate(&trial, tt.options, "config", "rubric", true)
			if err != nil || got != tt.want || store.saves != tt.wantSaves {
				t.Fatalf("shouldEvaluate = %v, %v; saves = %d", got, err, store.saves)
			}
			if tt.wantSaves > 0 {
				previous := trial.Judgments[1]
				if previous.Status != "error" || previous.Error == nil || previous.Error.Kind != "interrupted" || previous.CompletedAt == nil {
					t.Fatalf("interruption not persisted: %+v", previous)
				}
			}
		})
	}
}
