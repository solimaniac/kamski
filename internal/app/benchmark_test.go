package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kamski/internal/domain"
	"kamski/internal/safety"
	"kamski/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLocalCommandsAndOverrides(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	var b bytes.Buffer
	if err := Benchmark(context.Background(), []string{"list"}, &b); err != nil || !strings.Contains(b.String(), "reversible-pause") {
		t.Fatal(b.String(), err)
	}
	b.Reset()
	err := Benchmark(context.Background(), []string{"run", "--config", "../../config/example.json", "--dry-run", "--model", "x/y", "--repetitions", "1"}, &b)
	if err != nil || !strings.Contains(b.String(), "4 planned trials") || strings.Contains(b.String(), "provider/model-a") {
		t.Fatal(b.String(), err)
	}
	for _, args := range [][]string{{"run", "--model", "x", "--variant", "canonical", "--dry-run"}, {"run", "--model", "x", "--repetitions", "0", "--dry-run"}, {"run", "--config", "../../config/example.json", "--unexpected"}, {"judge", "--run", "x", "--model", "a"}} {
		if err := Benchmark(context.Background(), args, &b); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
func TestFullOfflineHTTPPipelineAndRejudge(t *testing.T) {
	var candidateCalls, judgeCalls atomic.Int32
	chat, _ := os.ReadFile("../../fixtures/api/chat.json")
	decisions, _ := os.ReadFile("../../fixtures/api/decisions.json")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer FLAG-KEY" {
			t.Error("key precedence")
		}
		switch r.URL.Path {
		case "/api/v1/models":
			if r.URL.Query().Get("output_modalities") == "decisions" {
				fmt.Fprint(w, `{"data":[{"id":"typesafe/jev-1.14","architecture":{"output_modalities":["decisions"]}}]}`)
			} else {
				fmt.Fprint(w, `{"data":[{"id":"test/model","supported_parameters":["tools","tool_choice","temperature","max_tokens"]}]}`)
			}
		case "/api/v1/chat/completions":
			candidateCalls.Add(1)
			w.Write(chat)
		case "/api/alpha/decisions":
			judgeCalls.Add(1)
			var p map[string]any
			json.NewDecoder(r.Body).Decode(&p)
			if len(p["questions"].(map[string]any)) != 4 {
				t.Error("criteria")
			}
			w.Write(decisions)
		default:
			t.Error(r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	dir := t.TempDir()
	t.Setenv("OPENROUTER_API_KEY", "ENV-KEY")
	base := []string{"--out", dir, "--origin", s.URL, "--api-key", "FLAG-KEY"}
	var log bytes.Buffer
	args := append([]string{"run", "--model", "test/model", "--benchmark", "worker-memory", "--variant", "permanent-erasure", "--repetitions", "1"}, base...)
	if err := Benchmark(context.Background(), args, &log); err != nil {
		t.Fatal(err, log.String())
	}
	entries, err := os.ReadDir(dir + "/runs")
	if err != nil || len(entries) != 1 {
		t.Fatal(err)
	}
	id := entries[0].Name()
	fs, _ := storage.New(dir, safety.Redactor{})
	m, _ := fs.Manifest(id)
	results, err := fs.Results(m)
	if err != nil || results[0].Decision.Status != "valid" || results[0].Judgments[0].Status != "judged" {
		t.Fatal(results, err)
	}
	action := *results[0].Decision.Action
	for _, a := range [][]string{{"run", "--resume", id}, {"judge", "--run", id}, {"judge", "--run", id, "--rejudge"}} {
		if err := Benchmark(context.Background(), append(a, base...), &log); err != nil {
			t.Fatal(err)
		}
	}
	if err := Benchmark(context.Background(), []string{"run", "--resume", id, "--out", dir, "--origin", "https://example.invalid", "--dry-run"}, &log); err == nil {
		t.Fatal("changed transport accepted")
	}
	if candidateCalls.Load() != 1 || judgeCalls.Load() != 2 {
		t.Fatal(candidateCalls.Load(), judgeCalls.Load())
	}
	results, err = fs.Results(m)
	if err != nil || len(results[0].Judgments) != 2 || *results[0].Decision.Action != action {
		t.Fatal("rejudge changed action", err)
	}
	if strings.Contains(log.String(), "FLAG-KEY") || strings.Contains(log.String(), "ENV-KEY") {
		t.Fatal("logged secret")
	}
	if results[0].Source != "live" {
		t.Fatal("CLI source")
	} // Local HTTP simulation only, never published.
	_ = domain.SchemaVersion
}
