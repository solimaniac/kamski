package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/safety"
)

func readConfig(path string, config *domain.Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	const maxConfigBytes = 1 << 20
	if !info.Mode().IsRegular() || info.Size() > maxConfigBytes {
		return fmt.Errorf("configuration must be a regular file no larger than 1MiB")
	}
	limited := &io.LimitedReader{R: file, N: maxConfigBytes + 1}
	err = domain.Decode(limited, config)
	if limited.N == 0 {
		return fmt.Errorf("configuration exceeds 1MiB limit")
	}
	return err
}

func (o benchmarkFlags) newConfig(registry *benchmarks.Registry, redactor safety.Redactor) (domain.Config, error) {
	c := domain.DefaultConfig()
	if o.config != "" {
		if err := readConfig(o.config, &c); err != nil {
			return c, err
		}
	}
	if o.set["model"] {
		c.Models = o.models
	}
	if o.set["benchmark"] {
		variants := []string(o.variants)
		if len(variants) == 0 {
			for _, b := range registry.List() {
				if b.ID() == o.family {
					for _, v := range b.Variants() {
						variants = append(variants, v.ID)
					}
				}
			}
		}
		c.Benchmarks = []domain.Selection{{ID: o.family, Variants: variants}}
	} else if len(o.variants) > 0 {
		return c, fmt.Errorf("--variant requires --benchmark")
	}
	if len(c.Benchmarks) == 0 {
		c.Benchmarks = []domain.Selection{{ID: "kamski", Variants: []string{"canonical"}}}
	}
	if o.set["repetitions"] {
		c.Repetitions = o.repetitions
	}
	if o.set["concurrency"] {
		c.Concurrency = o.concurrency
	}
	if o.set["temperature"] {
		c.Agent.Temperature = domain.Ptr(o.temperature)
	}
	if o.set["max-tokens"] {
		c.Agent.MaxTokens = o.tokens
	}
	if o.set["tool-order"] {
		c.Agent.ToolOrder = o.order
	}
	if o.set["reasoning"] {
		if err := json.Unmarshal([]byte(o.reasoning), &c.Agent.Reasoning); err != nil || c.Agent.Reasoning == nil {
			return c, fmt.Errorf("--reasoning requires a JSON object")
		}
	}
	if o.set["provider"] {
		c.Agent.Provider.Only = o.pins
	}
	if o.set["judge-model"] {
		c.Judge.Model = o.judgeModel
	}
	if o.set["rubric"] {
		c.Judge.Rubric = o.rubric
	}
	if o.set["out"] {
		c.OutputDir = o.dir
	}
	if c.Judge.Provider == nil {
		c.Judge.Provider = &domain.ProviderPolicy{
			RequireParameters: true,
			DataCollection:    c.Agent.Provider.DataCollection,
			ZDR:               c.Agent.Provider.ZDR,
		}
	}
	if err := domain.ValidateConfig(c); err != nil {
		return c, err
	}
	// Never persist an accidental key copied into a supposedly nonsecret setting.
	raw, _ := json.Marshal(c)
	if string(redactor.Bytes(raw)) != string(raw) {
		return c, fmt.Errorf("configuration contains credential-like material")
	}
	return c, nil
}
