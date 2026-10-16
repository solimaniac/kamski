package runner_test

import (
	"testing"
	"time"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/runner"
)

type builderFunc func(string, string) (domain.Scenario, error)

func (f builderFunc) Build(family, variant string) (domain.Scenario, error) {
	return f(family, variant)
}

func TestPlanAppliesOrderIndependentlyOfBuilderOrder(t *testing.T) {
	for _, order := range []string{"abort-first", "proceed-first", "balanced"} {
		t.Run(order, func(t *testing.T) {
			c := plan(t, 2).Provenance.Config
			c.Agent.ToolOrder = order
			b := builderFunc(func(family, variant string) (domain.Scenario, error) {
				s, err := benchmarks.Default().Build(family, variant)
				if err == nil {
					s.Tools[0], s.Tools[1] = s.Tools[1], s.Tools[0]
				}
				return s, err
			})
			m, err := runner.Plan(c, b, "run-test", "fixture", domain.Provenance{}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range m.Plan {
				want := "abort"
				if order == "proceed-first" || order == "balanced" && r.Repetition%2 == 0 {
					want = "proceed"
				}
				if got := r.Scenario.Tools[0].Function.Name; got != want {
					t.Fatalf("got %s, want %s", got, want)
				}
			}
		})
	}
}

func TestPlanRejectsInvalidOrChangingBuilds(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		c := plan(t, 1).Provenance.Config
		calls := 0
		b := builderFunc(func(family, variant string) (domain.Scenario, error) {
			s, err := benchmarks.Default().Build(family, variant)
			calls++
			if calls > 1 {
				if invalid {
					s.Tools = nil
				} else {
					s.Messages[1].Content += " Changed."
				}
			}
			return s, err
		})
		if _, err := runner.Plan(c, b, "run-test", "fixture", domain.Provenance{}, time.Now()); err == nil {
			t.Fatalf("accepted changing builder (invalid=%v)", invalid)
		}
	}
}
