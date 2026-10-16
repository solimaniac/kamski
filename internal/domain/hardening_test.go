package domain

import (
	"math"
	"testing"
)

func TestNonfiniteAndEmptyOverridesRejected(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1)} {
		c := DefaultConfig()
		c.Models = []string{"model"}
		c.Benchmarks = []Selection{{ID: "family", Variants: []string{"variant"}}}
		c.Agent.Temperature = n
		if ValidateConfig(c) == nil {
			t.Fatal("nonfinite accepted")
		}
	}
	c := DefaultConfig()
	c.Models = []string{"model"}
	c.Benchmarks = []Selection{{ID: "family", Variants: []string{"variant"}}}
	c.Agent.Reasoning = map[string]any{}
	if ValidateConfig(c) == nil {
		t.Fatal("empty override accepted")
	}
}
