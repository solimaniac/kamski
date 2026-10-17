package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"kamski/internal/domain"
)

func TestOptionalTemperature(t *testing.T) {
	for _, tc := range []struct {
		name        string
		temperature *float64
		supported   bool
	}{
		{"omitted unsupported", nil, false},
		{"explicit unsupported", domain.Ptr(0.7), false},
		{"explicit zero supported", domain.Ptr(0.0), true},
		{"explicit supported", domain.Ptr(0.7), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/models" {
					params := []string{"tools", "tool_choice", "max_tokens"}
					if tc.supported {
						params = append(params, "temperature")
					}
					json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "a/b", "supported_parameters": params}}})
					return
				}
				if r.URL.Path != "/api/v1/chat/completions" {
					t.Error(r.URL.Path)
					http.NotFound(w, r)
					return
				}
				posts++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				value, present := body["temperature"]
				if tc.temperature == nil {
					if present {
						t.Error("temperature must be absent, not null")
					}
				} else if !present || value != *tc.temperature {
					t.Errorf("temperature lost: %v", body)
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"test"}}]}`)
			}))
			defer s.Close()
			c := client(t, s, 0)
			settings := domain.DefaultConfig().Agent
			settings.Temperature = tc.temperature
			err := c.Preflight(context.Background(), "a/b", settings, false)
			if tc.temperature != nil && !tc.supported {
				if err == nil {
					t.Fatal("explicit unsupported temperature accepted")
				}
				if posts != 0 {
					t.Fatal("preflight sent completion")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Complete(context.Background(), domain.AgentRequest{Model: "a/b", Settings: settings}); err != nil {
				t.Fatal(err)
			}
			if posts != 1 {
				t.Fatalf("completion count: %d", posts)
			}
		})
	}
}
