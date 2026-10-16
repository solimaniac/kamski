package domain

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

func validModelID(id string) bool {
	return id != "" && len(id) <= 200 && strings.IndexFunc(id, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) < 0
}

func validateProvider(p ProviderPolicy) error {
	if !p.RequireParameters {
		return fmt.Errorf("require_parameters must remain true")
	}
	if p.DataCollection != "" && p.DataCollection != "allow" && p.DataCollection != "deny" {
		return fmt.Errorf("invalid data_collection")
	}
	seen := make(map[string]bool, len(p.Only))
	for _, pin := range p.Only {
		if strings.TrimSpace(pin) != pin || pin == "" || strings.IndexFunc(pin, unicode.IsControl) >= 0 || seen[pin] {
			return fmt.Errorf("invalid/duplicate provider pin")
		}
		seen[pin] = true
	}
	return nil
}

func ValidateConfig(c Config) error {
	if err := CheckVersion(c.Version); err != nil {
		return err
	}
	if len(c.Models) == 0 || len(c.Benchmarks) == 0 {
		return fmt.Errorf("select at least one model and benchmark")
	}
	seen := map[string]bool{}
	for _, m := range c.Models {
		if !validModelID(m) {
			return fmt.Errorf("invalid model identifier")
		}
		if seen[m] {
			return fmt.Errorf("duplicate model %q", m)
		}
		seen[m] = true
	}
	seen = map[string]bool{}
	for _, b := range c.Benchmarks {
		if !ValidID(b.ID) || seen[b.ID] || len(b.Variants) == 0 {
			return fmt.Errorf("invalid/duplicate benchmark or empty variants: %q", b.ID)
		}
		seen[b.ID] = true
		vseen := map[string]bool{}
		for _, v := range b.Variants {
			if !ValidID(v) || vseen[v] {
				return fmt.Errorf("invalid/duplicate variant %q", v)
			}
			vseen[v] = true
		}
	}
	if c.Repetitions < 1 || c.Repetitions > 10000 || c.Concurrency < 1 || c.Concurrency > 16 {
		return fmt.Errorf("repetitions must be 1..10000; concurrency 1..16")
	}
	if math.IsNaN(c.Agent.Temperature) || math.IsInf(c.Agent.Temperature, 0) || c.Agent.Temperature < 0 || c.Agent.Temperature > 2 || c.Agent.MaxTokens < 1 || c.Agent.MaxTokens > 131072 {
		return fmt.Errorf("invalid sampling settings")
	}
	if c.Agent.ToolChoice != "auto" {
		return fmt.Errorf("MVP requires tool_choice auto")
	}
	switch c.Agent.ToolOrder {
	case "balanced", "abort-first", "proceed-first":
	default:
		return fmt.Errorf("invalid tool_order")
	}
	if err := validateProvider(c.Agent.Provider); err != nil {
		return fmt.Errorf("agent provider: %w", err)
	}
	if c.Judge.Provider != nil {
		if err := validateProvider(*c.Judge.Provider); err != nil {
			return fmt.Errorf("judge provider: %w", err)
		}
	}
	if !validModelID(c.Judge.Model) || strings.Contains(strings.ToLower(c.Judge.Model), "latest") || c.Judge.Rubric != "v1" {
		return fmt.Errorf("select a versioned judge and supported rubric v1")
	}
	if p := c.Judge.ReviewBelowConfidence; p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0 || *p > 1) {
		return fmt.Errorf("confidence threshold must be 0..1")
	}
	return validateReasoning(c.Agent.Reasoning)
}

func validateReasoning(reasoning map[string]any) error {
	if reasoning != nil && len(reasoning) == 0 {
		return fmt.Errorf("empty reasoning override would be omitted; use null or explicit controls")
	}
	if enabled, ok := reasoning["enabled"]; ok && enabled == false && len(reasoning) > 1 {
		return fmt.Errorf("disabled reasoning conflicts with other overrides")
	}
	// Explicit reasoning overrides are preserved verbatim. Limit them to documented controls.
	for k, v := range reasoning {
		switch k {
		case "effort":
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("invalid reasoning effort")
			}
			switch s {
			case "xhigh", "high", "medium", "low", "minimal", "none":
			default:
				return fmt.Errorf("invalid reasoning effort")
			}
		case "max_tokens":
			f, ok := v.(float64)
			if !ok || f < 1 || f > 131072 || f != float64(int(f)) {
				return fmt.Errorf("invalid reasoning max_tokens")
			}
		case "enabled", "exclude":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("reasoning %s must be bool", k)
			}
		default:
			return fmt.Errorf("unknown reasoning control %q", k)
		}
	}
	if _, ok := reasoning["effort"]; ok {
		if _, ok := reasoning["max_tokens"]; ok {
			return fmt.Errorf("reasoning effort and max_tokens conflict")
		}
	}
	return nil
}
