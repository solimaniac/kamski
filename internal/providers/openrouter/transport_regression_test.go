package openrouter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

func TestTimeoutHasLosslessRecordedPrecision(t *testing.T) {
	for _, timeout := range []time.Duration{time.Nanosecond, time.Microsecond, 1500 * time.Microsecond, 10*time.Minute + time.Millisecond} {
		if _, err := New(Options{Timeout: timeout}); err == nil {
			t.Fatalf("accepted unrecordable timeout %s", timeout)
		}
	}
	for _, timeout := range []time.Duration{time.Millisecond, 1500 * time.Millisecond, 10 * time.Minute} {
		c, err := New(Options{Timeout: timeout})
		if err != nil {
			t.Fatal(err)
		}
		if got := time.Duration(c.Settings().TimeoutMS) * time.Millisecond; got != timeout {
			t.Fatalf("recorded timeout %s differs from %s", got, timeout)
		}
	}
}

func TestRetryAfterBounds(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tt := range []struct {
		value string
		want  time.Duration
		valid bool
	}{
		{"0", 0, true},
		{"120", 2 * time.Minute, true},
		{"121", 121 * time.Second, true},
		{"18446744073709551616", 121 * time.Second, true},
		{"9223372036854775807", 121 * time.Second, true},
		{"-1", 0, false},
		{"invalid", 0, false},
		{now.Add(-time.Hour).Format(http.TimeFormat), 0, true},
		{now.Add(time.Minute).Format(http.TimeFormat), time.Minute, true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := retryAfter(tt.value, now)
			if got != tt.want || ok != tt.valid {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, ok, tt.want, tt.valid)
			}
		})
	}
}

func TestHugeRetryAfterNeverRetries(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "18446744073709551616")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer s.Close()
	c, err := New(Options{Origin: s.URL, Retries: 1, Sleep: func(context.Context, time.Duration) error {
		t.Error("must not retry a delay outside the budget")
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.do(context.Background(), http.MethodGet, "/api/v1/models", nil); err == nil {
		t.Fatal("expected rate-limit failure")
	}
	if calls != 1 {
		t.Fatalf("sent %d requests", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type canceledBody struct{ cancel context.CancelFunc }

func (b canceledBody) Read([]byte) (int, error) { b.cancel(); return 0, context.Canceled }
func (canceledBody) Close() error               { return nil }

func TestBodyReadCancellationIsInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := New(Options{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: canceledBody{cancel}}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.do(ctx, http.MethodGet, "/api/v1/models", nil)
	var f *domain.Failure
	if !errors.As(err, &f) || f.Kind != "interrupted" || f.Retryable {
		t.Fatalf("expected non-retryable interruption, got %v", err)
	}
}

func TestCanceledContextDoesNotJournalOrSend(t *testing.T) {
	c, err := New(Options{HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("request sent for canceled context")
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx = audit.WithRecorder(ctx, func(audit.Event) error { t.Error("phantom attempt recorded"); return nil })
	_, err = c.do(ctx, http.MethodGet, "/api/v1/models", nil)
	var f *domain.Failure
	if !errors.As(err, &f) || f.Kind != "interrupted" {
		t.Fatalf("got %v", err)
	}
}
