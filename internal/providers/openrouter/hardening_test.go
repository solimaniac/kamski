package openrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"kamski/internal/audit"
	"kamski/internal/domain"
)

func TestRetryAfterAndMalformedRawRetention(t *testing.T) {
	var n atomic.Int32
	delay := time.Duration(-1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"code":429,"message":"wait"}}`)
			return
		}
		fmt.Fprint(w, "not-json SENTINEL-KEY")
	}))
	defer s.Close()
	c, _ := New(Options{Origin: s.URL, Key: "SENTINEL-KEY", Retries: 1, Sleep: func(ctx context.Context, d time.Duration) error { delay = d; return nil }})
	events := []audit.Event{}
	ctx := audit.WithRecorder(context.Background(), func(e audit.Event) error { events = append(events, e); return nil })
	_, err := c.Complete(ctx, domain.AgentRequest{})
	if err == nil || delay != 3*time.Second || n.Load() != 2 {
		t.Fatal(err, delay, n.Load())
	}
	last := events[len(events)-1]
	if last.ResponseText == nil || *last.ResponseText != "not-json [REDACTED]" {
		t.Fatal("malformed raw body lost", last)
	}
	var calls atomic.Int32
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer blocked.Close()
	c = client(t, blocked, 0)
	ctx = audit.WithRecorder(context.Background(), func(audit.Event) error { return errors.New("disk failure") })
	_, err = c.Complete(ctx, domain.AgentRequest{})
	var persist *audit.PersistenceError
	if !errors.As(err, &persist) || calls.Load() != 0 {
		t.Fatal("request sent before durable attempt", err, calls.Load())
	}
}
func TestMissingMessageAndProviderPinUnavailable(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"a/b","supported_parameters":["tools","tool_choice","temperature","max_tokens"]}]}`)
		case "/api/v1/models/a/b/endpoints":
			fmt.Fprint(w, `{"data":{"endpoints":[]}}`)
		default:
			fmt.Fprint(w, `{"choices":[{"finish_reason":"stop"}]}`)
		}
	}))
	defer s.Close()
	c := client(t, s, 0)
	cfg := domain.DefaultConfig().Agent
	cfg.Provider.Only = []string{"Pinned"}
	err := c.Preflight(context.Background(), "a/b", cfg, false)
	if err == nil || err.(*domain.Failure).Kind != "unavailable" {
		t.Fatal(err)
	}
	_, err = c.Complete(context.Background(), domain.AgentRequest{})
	if err == nil || err.(*domain.Failure).Kind != "protocol_error" {
		t.Fatal("missing message treated as model refusal", err)
	}
}
