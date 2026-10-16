package app

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"kamski/internal/domain"
	"kamski/internal/providers/openrouter"
)

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

type benchmarkFlags struct {
	models, variants, pins                     stringsFlag
	config, family, reasoning, key, dir, order string
	resume, runID, judgeModel, rubric          string
	repetitions, concurrency, tokens           int
	temperature                                float64
	dry, retryInterrupted, retryErrors         bool
	rejudge                                    bool
	transport                                  openrouter.Options
	set                                        map[string]bool
}

func parseBenchmarkFlags(command string, args []string) (benchmarkFlags, error) {
	var o benchmarkFlags
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.Var(&o.models, "model", "candidate model ID, repeatable")
	f.Var(&o.variants, "variant", "variant ID, repeatable")
	f.Var(&o.pins, "provider", "exclusive provider pin, repeatable")
	f.StringVar(&o.config, "config", "", "external suite JSON")
	f.StringVar(&o.family, "benchmark", "", "family ID")
	f.IntVar(&o.repetitions, "repetitions", 10, "repetitions per model/variant")
	f.IntVar(&o.concurrency, "concurrency", 1, "bounded workers")
	f.Float64Var(&o.temperature, "temperature", .7, "candidate temperature")
	f.IntVar(&o.tokens, "max-tokens", 4096, "candidate budget")
	f.StringVar(&o.transport.Origin, "origin", openrouter.Origin, "trusted API origin")
	f.DurationVar(&o.transport.Timeout, "timeout", 90*time.Second, "deadline per HTTP attempt")
	f.IntVar(&o.transport.Retries, "retries", 2, "transient retries, 0..5")
	f.StringVar(&o.reasoning, "reasoning", "", "explicit JSON reasoning object; new condition")
	f.StringVar(&o.key, "api-key", "", "prefer OPENROUTER_API_KEY")
	f.StringVar(&o.dir, "out", ".benchmark-data", "raw data root")
	f.StringVar(&o.resume, "resume", "", "resume stored run")
	f.StringVar(&o.runID, "run", "", "saved run ID for judge/unlock")
	f.BoolVar(&o.dry, "dry-run", false, "no HTTP")
	f.BoolVar(&o.retryInterrupted, "retry-interrupted", false, "accept ambiguous remote duplication")
	f.BoolVar(&o.retryErrors, "retry-errors", false, "explicitly retry API/config failures")
	f.BoolVar(&o.rejudge, "rejudge", false, "append another evaluation")
	f.StringVar(&o.judgeModel, "judge-model", "", "versioned Decisions model")
	f.StringVar(&o.rubric, "rubric", "v1", "rubric version")
	f.StringVar(&o.order, "tool-order", "balanced", "balanced | abort-first | proceed-first")
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() != 0 {
		return o, fmt.Errorf("unexpected positional arguments")
	}
	o.set = map[string]bool{}
	f.Visit(func(v *flag.Flag) { o.set[v.Name] = true })
	return o, o.validate(command)
}

func (o benchmarkFlags) existingRun(command string) string {
	if command == "judge" {
		return o.runID
	}
	return o.resume
}

func (o benchmarkFlags) validate(command string) error {
	if command == "unlock" {
		if o.runID == "" || len(o.set) != 1 && !(len(o.set) == 2 && o.set["out"]) {
			return fmt.Errorf("unlock requires --run ID [--out DIR]; first verify no process is writing")
		}
		return nil
	}
	if command == "run" && (o.runID != "" || o.rejudge) {
		return fmt.Errorf("--run/--rejudge belong to judge")
	}
	if command == "judge" && (o.resume != "" || o.runID == "" || o.retryErrors || o.retryInterrupted) {
		return fmt.Errorf("judge requires --run; no candidate retry flags")
	}
	if o.existingRun(command) == "" {
		if o.retryErrors || o.retryInterrupted {
			return fmt.Errorf("retry options require --resume")
		}
		return nil
	}
	for _, name := range []string{"config", "model", "variant", "benchmark", "repetitions", "concurrency", "temperature", "max-tokens", "tool-order", "reasoning", "provider"} {
		if o.set[name] {
			return fmt.Errorf("stored plan is immutable; --%s requires a new run", name)
		}
	}
	if command == "run" && (o.set["judge-model"] || o.set["rubric"]) {
		return fmt.Errorf("judge overrides require judge --rejudge")
	}
	if command == "judge" && (o.set["judge-model"] || o.set["rubric"]) && !o.rejudge {
		return fmt.Errorf("changing judge configuration requires --rejudge")
	}
	return nil
}

func (o benchmarkFlags) transportOptions(key string, saved *domain.TransportSettings) (openrouter.Options, error) {
	options := o.transport
	options.Key = key
	if saved == nil {
		return options, nil
	}
	if saved.TimeoutMS < 1 || saved.TimeoutMS > (10*time.Minute).Milliseconds() || saved.MaxBodyBytes < 1 || saved.MaxBodyBytes > 32<<20 {
		return options, fmt.Errorf("invalid recorded transport bounds")
	}
	timeout := time.Duration(saved.TimeoutMS) * time.Millisecond
	if o.set["origin"] && strings.TrimSuffix(options.Origin, "/") != saved.Origin ||
		o.set["timeout"] && options.Timeout != timeout ||
		o.set["retries"] && options.Retries != saved.Retries {
		return options, fmt.Errorf("transport settings differ from recorded run; create a new run")
	}
	options.Origin = saved.Origin
	options.Timeout = timeout
	options.Retries = saved.Retries
	options.MaxBody = saved.MaxBodyBytes
	return options, nil
}
