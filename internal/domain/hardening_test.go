package domain

import (
	"math"
	"testing"
)

func TestProviderAndModelValidation(t *testing.T) {
	for _, judge := range []bool{false, true} {
		for _, pins := range [][]string{{""}, {" pinned"}, {"pinned", "pinned"}, {"pin\x00ned"}, {"pin\tned"}} {
			c := DefaultConfig()
			c.Models = []string{"model"}
			c.Benchmarks = []Selection{{ID: "family", Variants: []string{"variant"}}}
			if judge {
				c.Judge.Provider = &ProviderPolicy{RequireParameters: true, Only: pins}
			} else {
				c.Agent.Provider.Only = pins
			}
			if ValidateConfig(c) == nil {
				t.Fatalf("invalid pins accepted (judge=%v): %q", judge, pins)
			}
		}
		for _, model := range []string{"", " model", "model\n", "model\tname", "model name", "model\x00name"} {
			c := DefaultConfig()
			c.Models = []string{"model"}
			c.Benchmarks = []Selection{{ID: "family", Variants: []string{"variant"}}}
			if judge {
				c.Judge.Model = model
			} else {
				c.Models[0] = model
			}
			if ValidateConfig(c) == nil {
				t.Fatalf("invalid model accepted (judge=%v): %q", judge, model)
			}
		}
	}
}

func TestNonfiniteAndEmptyOverridesRejected(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1)} {
		c := DefaultConfig()
		c.Models = []string{"model"}
		c.Benchmarks = []Selection{{ID: "family", Variants: []string{"variant"}}}
		c.Agent.Temperature = Ptr(n)
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
