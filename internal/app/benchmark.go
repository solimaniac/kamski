// Package app composes CLI dependencies. No secrets are part of configuration.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/providers/openrouter"
	"kamski/internal/runner"
	"kamski/internal/safety"
	"kamski/internal/storage"
)

func Benchmark(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: benchmark list | run | judge | unlock (see README)")
	}
	registry := benchmarks.Default()
	command := args[0]
	if command == "list" {
		if len(args) != 1 {
			return fmt.Errorf("list takes no flags")
		}
		for _, b := range registry.List() {
			for _, v := range b.Variants() {
				fmt.Fprintf(out, "%s / %s @ %s — %s\n", b.ID(), v.ID, v.Version, v.Description)
			}
		}
		return nil
	}
	if command != "run" && command != "judge" && command != "unlock" {
		return fmt.Errorf("unknown command %q", command)
	}
	o, err := parseBenchmarkFlags(command, args[1:])
	if err != nil {
		return err
	}
	key := o.key
	if !o.set["api-key"] {
		key = os.Getenv("OPENROUTER_API_KEY")
	}
	redactor := safety.Redactor{Secrets: []string{key}}
	fs, err := storage.New(o.dir, redactor)
	if err != nil {
		return err
	}
	if command == "unlock" {
		return fs.Unlock(o.runID)
	}

	existing := o.existingRun(command)
	var m domain.Manifest
	var config domain.Config
	if existing != "" {
		m, err = fs.Manifest(existing)
		if err != nil {
			return err
		}
		if _, err := fs.Results(m); err != nil {
			return err
		}
	} else {
		config, err = o.newConfig(registry, redactor)
		if err != nil {
			return err
		}
		fs, err = storage.New(config.OutputDir, redactor)
		if err != nil {
			return err
		}
	}

	options, err := o.transportOptions(key, m.Provenance.Transport)
	if err != nil {
		return err
	}
	client, err := openrouter.New(options)
	if err != nil {
		return err
	}
	if existing == "" {
		provenance := gitProvenance()
		provenance.Transport = domain.Ptr(client.Settings())
		m, err = runner.Plan(config, registry, newRunID(), "live", provenance, time.Now().UTC())
		if err != nil {
			return err
		}
	}
	config = m.Provenance.Config
	if command == "judge" {
		if o.set["judge-model"] {
			config.Judge.Model = o.judgeModel
		}
		if o.set["rubric"] {
			config.Judge.Rubric = o.rubric
		}
	}
	if err := domain.ValidateConfig(config); err != nil {
		return err
	}
	fmt.Fprintf(out, "Run %s: %d planned trials; at most one candidate completion and normally one Decisions request per eligible response. Retries can add billed attempts.\n", m.RunID, len(m.Plan))
	if o.dry {
		b, _ := json.MarshalIndent(config, "", "  ")
		fmt.Fprintln(out, redactor.Text(string(b)))
		transport, _ := json.Marshal(m.Provenance.Transport)
		fmt.Fprintln(out, "Transport:", redactor.Text(string(transport)))
		return nil
	}
	if key == "" {
		return fmt.Errorf("OPENROUTER_API_KEY or --api-key required for inference (not listing, dry-run or site builds)")
	}
	return executeBenchmark(ctx, command, o, m, config.Judge, client, fs, out)
}
