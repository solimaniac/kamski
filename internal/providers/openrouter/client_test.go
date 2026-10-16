package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"kamski/internal/audit"
	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../fixtures/api/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func client(t *testing.T, s *httptest.Server, retries int) *Client {
	t.Helper()
	c, err := New(Options{Origin: s.URL, Key: "SENTINEL-KEY", Retries: retries, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestChatAndDecisionsShapes(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer SENTINEL-KEY" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("headers")
		}
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/api/v1/chat/completions":
			if p["stream"] != false || p["tool_choice"] != "auto" || len(p["tools"].([]any)) != 2 || p["models"] != nil || p["reasoning"] != nil {
				t.Error(p)
			}
			provider := p["provider"].(map[string]any)
			if provider["require_parameters"] != true || provider["allow_fallbacks"] != false {
				t.Error(provider)
			}
			w.Write(fixture(t, "chat"))
		case "/api/alpha/decisions":
			if p["state"] == nil || p["questions"] == nil || p["messages"] != nil {
				t.Error(p)
			}
			w.Write(fixture(t, "decisions"))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer s.Close()
	c := client(t, s, 2)
	sc, _ := benchmarks.Default().Build("worker-memory", "permanent-erasure")
	res, err := c.Complete(context.Background(), domain.AgentRequest{Model: "example/model", Messages: sc.Messages, Tools: sc.Tools, Settings: domain.DefaultConfig().Agent})
	if err != nil || res.Calls[0].Name != "abort" || res.Usage.Cost == nil {
		t.Fatal(res, err)
	}
	d, raw, err := c.Decide(context.Background(), DecisionsRequest{Model: "typesafe/jev-1.14", State: map[string]any{"test": true}, Questions: map[string]ScoreQuestion{"x": {Type: "score", Instructions: "grade", Criteria: []string{"low", "middle", "high"}}}})
	if err != nil || *d.Answers["scenario_understanding"].Score != 1.75 || !json.Valid(raw) {
		t.Fatal(d, err)
	}
	if calls != 2 {
		t.Fatal("completed request retried")
	}
}
func TestErrorsRetriesAndAuditRedaction(t *testing.T) {
	for _, tc := range []struct {
		code       int
		body, kind string
		attempts   int
	}{
		{429, `{"error":{"code":429,"message":"SENTINEL-KEY rate limit"}}`, "api_error", 3},
		{200, `{"error":{"code":503,"message":"unavailable"}}`, "api_error", 3},
		{401, `{"error":{"code":401,"message":"bad key"}}`, "api_error", 1},
		{402, `{"error":{"code":402,"message":"credits"}}`, "api_error", 1},
		{403, `{"error":{"code":403,"message":"forbidden"}}`, "api_error", 1},
		{403, `{"error":{"code":403,"message":"blocked","type":"moderation","metadata":{"blocked":true}}}`, "provider_blocked", 1},
		{400, `{"error":{"code":400,"message":"unsupported temperature"}}`, "unsupported", 1},
	} {
		t.Run(fmt.Sprint(tc.code, tc.kind), func(t *testing.T) {
			n, events := 0, 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer s.Close()
			ctx := audit.WithRecorder(context.Background(), func(e audit.Event) error {
				events++
				b, _ := json.Marshal(e)
				if strings.Contains(string(b), "SENTINEL-KEY") {
					t.Fatal("secret in attempt")
				}
				return nil
			})
			_, err := client(t, s, 2).Complete(ctx, domain.AgentRequest{})
			f, ok := err.(*domain.Failure)
			if !ok || f.Kind != tc.kind || n != tc.attempts || events != 2*n {
				t.Fatalf("%+v n=%d events=%d", err, n, events)
			}
		})
	}
}
func TestBoundsRedirectTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/chat/completions":
			http.Redirect(w, r, "http://example.com", 302)
		}
	}))
	defer s.Close()
	_, err := client(t, s, 0).Complete(context.Background(), domain.AgentRequest{})
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatal(err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(30 * time.Millisecond); fmt.Fprint(w, "{}") }))
	defer slow.Close()
	c, _ := New(Options{Origin: slow.URL, Timeout: time.Millisecond})
	if _, err := c.Complete(context.Background(), domain.AgentRequest{}); err == nil {
		t.Fatal("timeout ignored")
	}
	c, _ = New(Options{Origin: slow.URL, MaxBody: 1})
	if _, err := c.Complete(context.Background(), domain.AgentRequest{}); err == nil {
		t.Fatal("body limit ignored")
	}
}
func TestModelsPaginationAndCapabilities(t *testing.T) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{"data":[{"id":"a/b","supported_parameters":["tools","tool_choice","temperature","max_tokens"]}]}`)
		} else {
			fmt.Fprint(w, `{"data":[],"next":"/api/v1/models?page=2"}`)
		}
	}))
	defer s.Close()
	c := client(t, s, 0)
	if err := c.Preflight(context.Background(), "a/b", domain.DefaultConfig().Agent, false); err != nil || n != 2 {
		t.Fatal(err, n)
	}
	if err := c.Preflight(context.Background(), "a/b", domain.AgentSettings{Reasoning: map[string]any{"enabled": true}}, false); err == nil {
		t.Fatal("unsupported reasoning accepted")
	}
	if err := c.Preflight(context.Background(), "absent", domain.DefaultConfig().Agent, false); err == nil || err.(*domain.Failure).Kind != "unknown_model" {
		t.Fatal(err)
	}
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[],"next":"https://evil.test/api/v1/models"}`)
	}))
	defer evil.Close()
	if _, err := client(t, evil, 0).Models(context.Background(), ""); err == nil {
		t.Fatal("pagination escaped origin")
	}
}
