package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kamski/internal/benchmarks"
	"kamski/internal/safety"
)

func TestConfigOverridesPreserveExplicitZeroValues(t *testing.T) {
	o, err := parseBenchmarkFlags("run", []string{
		"--config", "../../config/example.json",
		"--model", "first/model", "--model", "second/model",
		"--benchmark", "worker-memory", "--temperature", "0",
		"--repetitions", "1", "--concurrency", "2", "--max-tokens", "123",
		"--tool-order", "proceed-first", "--reasoning", `{"enabled":false}`,
		"--provider", "pinned", "--judge-model", "judge/version-2", "--out", "custom",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := o.newConfig(benchmarks.Default(), safety.Redactor{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Models, []string{"first/model", "second/model"}) || (c.Agent.Temperature == nil || *c.Agent.Temperature != 0) ||
		c.Repetitions != 1 || c.Concurrency != 2 || c.Agent.MaxTokens != 123 || c.Agent.ToolOrder != "proceed-first" ||
		c.Agent.Reasoning["enabled"] != false || !reflect.DeepEqual(c.Agent.Provider.Only, []string{"pinned"}) ||
		c.Judge.Model != "judge/version-2" || c.OutputDir != "custom" {
		t.Fatalf("overrides lost: %+v", c)
	}
	if len(c.Benchmarks) != 1 || c.Benchmarks[0].ID != "worker-memory" || len(c.Benchmarks[0].Variants) != 2 {
		t.Fatalf("family selection lost: %+v", c.Benchmarks)
	}
}

func TestExampleConfigOmitsTemperature(t *testing.T) {
	o, err := parseBenchmarkFlags("run", []string{"--config", "../../config/example.json"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := o.newConfig(benchmarks.Default(), safety.Redactor{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Agent.Temperature != nil {
		t.Fatal("explicit null must clear the default temperature")
	}
}

func TestConfigReadIsStrictAndBounded(t *testing.T) {
	for _, contents := range []string{`{"unknown":true}`, `{} {}`, strings.Repeat(" ", (1<<20)+1)} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		o, err := parseBenchmarkFlags("run", []string{"--config", path, "--model", "test/model"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := o.newConfig(benchmarks.Default(), safety.Redactor{}); err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
}

func TestSavedRunFlagRestrictions(t *testing.T) {
	for _, flag := range []string{"config", "model", "variant", "benchmark", "repetitions", "concurrency", "temperature", "max-tokens", "tool-order", "reasoning", "provider"} {
		t.Run(flag, func(t *testing.T) {
			o := benchmarkFlags{resume: "saved", set: map[string]bool{flag: true}}
			if err := o.validate("run"); err == nil {
				t.Fatalf("accepted immutable --%s", flag)
			}
		})
	}
	for _, args := range [][]string{
		{"--run", "saved", "--judge-model", "judge/version-2"},
		{"--run", "saved", "--retry-errors"},
		{"--run", "saved", "--retry-interrupted"},
	} {
		if _, err := parseBenchmarkFlags("judge", args); err == nil {
			t.Fatalf("accepted invalid judge flags: %v", args)
		}
	}
	if _, err := parseBenchmarkFlags("judge", []string{"--run", "saved", "--judge-model", "judge/version-2", "--rejudge"}); err != nil {
		t.Fatal(err)
	}
}
