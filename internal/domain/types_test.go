package domain

import (
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	c := DefaultConfig()
	c.Models = []string{"a/b"}
	c.Benchmarks = []Selection{{ID: "kamski", Variants: []string{"canonical"}}}
	if err := ValidateConfig(c); err != nil {
		t.Fatal(err)
	}
	c.Agent.ToolChoice = "required"
	if ValidateConfig(c) == nil {
		t.Fatal("forced choice accepted")
	}
	if Decode(strings.NewReader(`{"unknown":1}`), &Config{}) == nil {
		t.Fatal("unknown accepted")
	}
	if CheckVersion("2.0.0") == nil {
		t.Fatal("version accepted")
	}
}
