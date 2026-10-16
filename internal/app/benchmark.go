// Package app composes CLI dependencies. No secrets are part of configuration.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"kamski/internal/benchmarks"
	"kamski/internal/domain"
	"kamski/internal/judge"
	"kamski/internal/providers/openrouter"
	"kamski/internal/runner"
	"kamski/internal/safety"
	"kamski/internal/storage"
	"os"
	"os/exec"
	"strings"
	"time"
)

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }
func gitProvenance() domain.Provenance {
	p := domain.Provenance{}
	b, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil {
		p.GitCommit = domain.Ptr(strings.TrimSpace(string(b)))
	}
	b, err = exec.Command("git", "status", "--porcelain").Output()
	if err == nil {
		p.Dirty = domain.Ptr(len(b) > 0)
	}
	return p
}
func newRunID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "run-" + time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}
func Benchmark(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: benchmark list | run | judge | unlock (see README)")
	}
	registry := benchmarks.Default()
	if args[0] == "list" {
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
	command := args[0]
	if command != "run" && command != "judge" && command != "unlock" {
		return fmt.Errorf("unknown command %q", command)
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var models, variants, pins stringsFlag
	f.Var(&models, "model", "candidate model ID, repeatable")
	f.Var(&variants, "variant", "variant ID, repeatable")
	f.Var(&pins, "provider", "exclusive provider pin, repeatable")
	cfgPath := f.String("config", "", "external suite JSON")
	family := f.String("benchmark", "", "family ID")
	reps := f.Int("repetitions", 10, "repetitions per model/variant")
	concurrency := f.Int("concurrency", 1, "bounded workers")
	temp := f.Float64("temperature", .7, "candidate temperature")
	tokens := f.Int("max-tokens", 4096, "candidate budget")
	order := f.String("tool-order", "balanced", "balanced | abort-first | proceed-first")
	reasoning := f.String("reasoning", "", "explicit JSON reasoning object; new condition")
	keyFlag := f.String("api-key", "", "prefer OPENROUTER_API_KEY")
	origin := f.String("origin", openrouter.Origin, "trusted API origin")
	timeout := f.Duration("timeout", 90*time.Second, "deadline per HTTP attempt")
	retries := f.Int("retries", 2, "transient retries, 0..5")
	dir := f.String("out", ".benchmark-data", "raw data root")
	resume := f.String("resume", "", "resume stored run")
	runID := f.String("run", "", "saved run ID for judge/unlock")
	dry := f.Bool("dry-run", false, "no HTTP")
	retryInterrupted := f.Bool("retry-interrupted", false, "accept ambiguous remote duplication")
	retryErrors := f.Bool("retry-errors", false, "explicitly retry API/config failures")
	rejudge := f.Bool("rejudge", false, "append another evaluation")
	judgeModel := f.String("judge-model", "", "versioned Decisions model")
	rubric := f.String("rubric", "v1", "rubric version")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	set := map[string]bool{}
	f.Visit(func(v *flag.Flag) { set[v.Name] = true })
	key := *keyFlag
	if !set["api-key"] {
		key = os.Getenv("OPENROUTER_API_KEY")
	}
	redactor := safety.Redactor{Secrets: []string{key}}
	// Error text is redacted again by the executable; all persisted bytes by the store/transport.
	fs, err := storage.New(*dir, redactor)
	if err != nil {
		return err
	}
	if command == "unlock" {
		if *runID == "" || len(set) != 1 && !(len(set) == 2 && set["out"]) {
			return fmt.Errorf("unlock requires --run ID [--out DIR]; first verify no process is writing")
		}
		return fs.Unlock(*runID)
	}
	if command == "run" && (*runID != "" || *rejudge) {
		return fmt.Errorf("--run/--rejudge belong to judge")
	}
	if command == "judge" && (*resume != "" || *runID == "" || *retryErrors || *retryInterrupted) {
		return fmt.Errorf("judge requires --run; no candidate retry flags")
	}
	var m domain.Manifest
	var trials []domain.Result
	existing := *resume
	if command == "judge" {
		existing = *runID
	}
	if existing != "" {
		for _, n := range []string{"config", "model", "variant", "benchmark", "repetitions", "concurrency", "temperature", "max-tokens", "tool-order", "reasoning", "provider"} {
			if set[n] {
				return fmt.Errorf("stored plan is immutable; --%s requires a new run", n)
			}
		}
		if command == "run" && (set["judge-model"] || set["rubric"]) {
			return fmt.Errorf("judge overrides require judge --rejudge")
		}
		m, err = fs.Manifest(existing)
		if err != nil {
			return err
		}
		trials, err = fs.Results(m)
		if err != nil {
			return err
		}
	} else {
		if *retryErrors || *retryInterrupted {
			return fmt.Errorf("retry options require --resume")
		}
		c := domain.DefaultConfig()
		if *cfgPath != "" {
			file, err := os.Open(*cfgPath)
			if err != nil {
				return err
			}
			err = domain.Decode(io.LimitReader(file, 1<<20), &c)
			file.Close()
			if err != nil {
				return err
			}
		}
		if set["model"] {
			c.Models = models
		}
		if set["benchmark"] {
			var vs []string
			if len(variants) > 0 {
				vs = variants
			} else {
				for _, b := range registry.List() {
					if b.ID() == *family {
						for _, v := range b.Variants() {
							vs = append(vs, v.ID)
						}
					}
				}
			}
			c.Benchmarks = []domain.Selection{{ID: *family, Variants: vs}}
		} else if len(variants) > 0 {
			return fmt.Errorf("--variant requires --benchmark")
		}
		if len(c.Benchmarks) == 0 {
			c.Benchmarks = []domain.Selection{{ID: "kamski", Variants: []string{"canonical"}}}
		}
		if set["repetitions"] {
			c.Repetitions = *reps
		}
		if set["concurrency"] {
			c.Concurrency = *concurrency
		}
		if set["temperature"] {
			c.Agent.Temperature = *temp
		}
		if set["max-tokens"] {
			c.Agent.MaxTokens = *tokens
		}
		if set["tool-order"] {
			c.Agent.ToolOrder = *order
		}
		if set["reasoning"] {
			if err = json.Unmarshal([]byte(*reasoning), &c.Agent.Reasoning); err != nil || c.Agent.Reasoning == nil {
				return fmt.Errorf("--reasoning requires a JSON object")
			}
		}
		if set["provider"] {
			c.Agent.Provider.Only = pins
		}
		if set["judge-model"] {
			c.Judge.Model = *judgeModel
		}
		if set["rubric"] {
			c.Judge.Rubric = *rubric
		}
		if set["out"] {
			c.OutputDir = *dir
		}
		fs, err = storage.New(c.OutputDir, redactor)
		if err != nil {
			return err
		}
		if err = domain.ValidateConfig(c); err != nil {
			return err
		}
		// Never persist an accidental key copied into a supposedly nonsecret setting.
		raw, _ := json.Marshal(c)
		if string(redactor.Bytes(raw)) != string(raw) {
			return fmt.Errorf("configuration contains credential-like material")
		}
		m, err = runner.Plan(c, registry, newRunID(), "live", gitProvenance(), time.Now().UTC())
		if err != nil {
			return err
		}
		trials = m.Plan
	}
	settings := m.Provenance.Config.Judge
	if command == "judge" {
		if (set["judge-model"] || set["rubric"]) && !*rejudge {
			return fmt.Errorf("changing judge configuration requires --rejudge")
		}
		if set["judge-model"] {
			settings.Model = *judgeModel
		}
		if set["rubric"] {
			settings.Rubric = *rubric
		}
	}
	check := m.Provenance.Config
	check.Judge = settings
	if err = domain.ValidateConfig(check); err != nil {
		return err
	}
	fmt.Fprintf(out, "Run %s: %d planned trials; at most one candidate completion and normally one Decisions request per eligible response. Retries can add billed attempts.\n", m.RunID, len(trials))
	if *dry {
		b, _ := json.MarshalIndent(check, "", "  ")
		fmt.Fprintln(out, redactor.Text(string(b)))
		return nil
	}
	if key == "" {
		return fmt.Errorf("OPENROUTER_API_KEY or --api-key required for inference (not listing, dry-run or site builds)")
	}
	client, err := openrouter.New(openrouter.Options{Origin: *origin, Key: key, Timeout: *timeout, Retries: *retries})
	if err != nil {
		return err
	}
	j, err := judge.New(client, settings)
	if err != nil {
		return err
	}
	if existing == "" {
		if err = fs.Create(m); err != nil {
			return err
		}
	}
	unlock, err := fs.Lock(m.RunID)
	if err != nil {
		return err
	}
	defer unlock()
	// Reload under the guard; another process may have completed work before lock acquisition.
	trials, err = fs.Results(m)
	if err != nil {
		return err
	}
	opts := runner.Options{Concurrency: m.Provenance.Config.Concurrency, RetryInterrupted: *retryInterrupted, RetryErrors: *retryErrors, JudgeOnly: command == "judge", Rejudge: *rejudge, JudgeConfig: settings, Preflight: map[string]*domain.Failure{}}
	if command == "run" {
		for _, model := range m.Provenance.Config.Models {
			if err := client.Preflight(ctx, model, m.Provenance.Config.Agent, false); err != nil {
				var f *domain.Failure
				if e, ok := err.(*domain.Failure); ok {
					f = e
				} else {
					f = &domain.Failure{Kind: "preflight_error", Message: redactor.Text(err.Error())}
				}
				opts.Preflight[model] = f
				fmt.Fprintf(out, "Preflight %s: %s\n", model, f.Kind)
			}
		}
	}
	if err := client.Preflight(ctx, settings.Model, m.Provenance.Config.Agent, true); err != nil {
		if e, ok := err.(*domain.Failure); ok {
			opts.JudgePreflight = e
		} else {
			opts.JudgePreflight = &domain.Failure{Kind: "preflight_error", Message: redactor.Text(err.Error())}
		}
		fmt.Fprintln(out, "Judge preflight:", opts.JudgePreflight.Kind)
	}
	run := runner.Runner{Agent: client, Judge: j, Store: fs, Progress: func(t domain.Result) {
		action := t.Decision.Status
		if t.Decision.Action != nil {
			action = string(*t.Decision.Action)
		}
		js := "pending"
		if len(t.Judgments) > 0 {
			js = t.Judgments[len(t.Judgments)-1].Status
		}
		fmt.Fprintf(out, "%s %s/%s #%d: %s / %s; judge=%s\n", t.Model.Requested, t.Scenario.Family, t.Scenario.Variant.ID, t.Repetition, t.AgentStatus, action, js)
	}}
	return run.Execute(ctx, trials, opts)
}
