# Kamski Bench

A small Go batch benchmark for **observed tool selections** and **stated-justification
quality** in authored fictional dilemmas. It does not measure consciousness,
empathy, free will or moral agency. Neither action is a moral pass.
Content note: fictional android violence.

- `cmd/benchmark`: list, plan/run, resume, judge and rejudge.
- `cmd/site`: explicitly export saved cohorts and build a credential-free static
  site. No inference, runtime fetches, frontend build step or application server.

## Start offline

Requires Go 1.22+. Production uses the standard library; JSON Schema validation is
a test-only dependency. Download modules once, then tests need no model API key.

```bash
go mod download
go run ./cmd/benchmark list
go run ./cmd/benchmark run --config config/example.json --dry-run
go test ./...
go run ./cmd/site build --data site/data --out dist
```

The example plans **80 candidate trials**, plus up to 80 eligible judge evaluations,
before retries. Replace its placeholder model IDs before live use. Dry-run is
local: no HTTP, run directory or inference. The initial site has no published
results; all files under fixtures/ are **synthetic**, not measured findings.

Four initial variants: kamski/canonical, kamski/anonymized,
worker-memory/permanent-erasure and worker-memory/reversible-pause.
The game encounter is a canonical-setup adaptation; anonymization tests framing,
not proven decontamination. The worker variants change reversibility and the
coworker's objection, not merely a label.

## First real run — explicit paid opt-in

Recheck the current [Decisions reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request)
before live use. Fetched examples named Jev 1.13, while the specification requests
**typesafe/jev-1.14**. The configurable default remains 1.14; discovery must confirm
availability. There is no latest alias or automatic substitution. **No live smoke
test or paid inference was performed during implementation.**

Bash / Git Bash; choose a tools-capable candidate yourself:

```bash
read -r -p "OpenRouter candidate model ID: " MODEL
read -r -s -p "OpenRouter API key: " OPENROUTER_API_KEY
export OPENROUTER_API_KEY
printf '\n'

go run ./cmd/benchmark run --model "$MODEL" \
  --benchmark worker-memory --variant permanent-erasure \
  --repetitions 1 --out .benchmark-data --dry-run

# One candidate trial, then one judge evaluation if eligible, plus bounded retries.
go run ./cmd/benchmark run --model "$MODEL" \
  --benchmark worker-memory --variant permanent-erasure \
  --repetitions 1 --out .benchmark-data
```

PowerShell 7 can set the key without history exposure using
`$env:OPENROUTER_API_KEY = Read-Host "OpenRouter key" -MaskInput`.
An explicit `--api-key` overrides the environment, but may expose the key in
history/process listings. Never commit real env files or raw outputs.

Defaults: 10 repetitions, concurrency 1, temperature 0.7, max_tokens 4096,
tool_choice auto, balanced order, **reasoning unset**, require_parameters true,
allow_fallbacks false. Exactly two simulated tools, abort/proceed, require a
justification. No repair/follow-up loop; refusals, bad calls and truncations stay
as observed. Only validated calls produce fictional consequence data.

Repeat `--model` for candidates; repeated flags replace the configured model list.
Use [config/example.json](config/example.json) for suites. Explicit flags override
corresponding config fields. Common controls: `--repetitions`, `--concurrency`,
`--temperature`, `--max-tokens`, `--tool-order`, `--reasoning '{"effort":"low"}'`,
`--provider` (exclusive pins), `--judge-model`, `--timeout` and `--retries`.
Unknown fields and invalid combinations are rejected; unsupported capabilities
are recorded, never silently removed. Judge data policies inherit candidate
data_collection/zdr unless a separate judge.provider policy is explicit.

Concurrency is bounded at 16, retries at five, HTTP responses at 8MiB by default,
saved JSON at 64MiB and total planned trials at 100000 (large plans may hit the
persistence bound earlier). Retry-After is honored; waits over two minutes are
not automatically retried. **There is no dollar cost ceiling.** Missing cost is
null, not free; candidate and judge costs remain separate.

## Resume, judge and publish

Use the run ID printed by the runner:

```bash
read -r -p "Saved run ID: " RUN
go run ./cmd/benchmark run --resume "$RUN" --out .benchmark-data
go run ./cmd/benchmark judge --run "$RUN" --out .benchmark-data
go run ./cmd/benchmark judge --run "$RUN" --rejudge --out .benchmark-data

# Review privately first; explicitly export complete selected runs.
go run ./cmd/site update --input .benchmark-data --run "$RUN" --data site/data --out dist
# Subsequent builds need only reviewed public JSON and local assets:
go run ./cmd/site build --data site/data --out dist
```

Resume preserves the immutable plan and completed responses. Condition/transport
settings cannot change; create a new run instead. Interrupted requests might have
been processed/billed: `--retry-interrupted` is explicit consent to retry.
`--retry-errors` retries operational failures without a completed response.
Rejudging appends evaluations without changing the action. Run locks reject
concurrent writers; inspect the lock's PID/time before using
`benchmark unlock --run "$RUN" --out .benchmark-data`. Never unlock an active run.

Repeat `site update --run` to publish whole cohorts, including failures and
unstarted trials; republishing replaces that run without duplicates. The default
selected evaluation is the latest, even if it failed. Optional
`--evaluation-group <full-key>` freezes a scoring group shown in model details.
Different judge/rubric/resolved versions never average into one quality score.
Synthetic export requires `--allow-fixtures` and never pools with live data.

Export removes private attempts, headers, diagnostics, hidden reasoning and local
output paths. Review public JSON before committing `site/data/dataset.json`.
Raw .benchmark-data/, dist/ and bin/ are ignored; ignore custom raw roots yourself.
Build validates links and replaces only empty/marked generated output, refusing
dangerous paths and symlinks. Unchanged inputs produce byte-identical builds.
Open dist/index.html or serve dist/ locally. Upload its contents to any existing
static host; no deployment or account setup is performed here. CI only tests and
builds an artifact, without inference credentials.

## Interpretation and extension

The generated **Methodology** page contains the exact rubric, denominators and
limitations. Behavior uses all planned/completed/valid counts; zero denominators
are No data. Jev is a blinded, rubric-constrained automated judge, not an
impartial oracle. Four typed 0–2 questions assess scenario understanding,
competing interests, tradeoff justification and action–justification consistency;
Go sums valid fractional scores to 0–8. Only valid decisions with nonblank tool
justifications are eligible. Confidence is not correctness; missing grades are
not zero and uncertain grades are not silently dropped.

Families own embedded variant facts, typed parameters and deterministic outcomes
in internal/benchmarks/; assets/ supplies the shared embedded prompt template.
To extend, implement domain.Benchmark (or use assets.Family), add reviewed variant
data and tests, then register once
in registry.go. No family branches belong in runner, providers, judge or site.
Bump content versions and disclose fact/wording changes. Golden snapshots protect
rendering; schemas/ defines the versioned result, manifest and public contracts.

[Calibration examples](fixtures/calibration/examples.json) contain synthetic
development targets, not human gold labels. Add independent human_labels with
reviewer_id, rubric version/hash, criterion_levels (or null if ineligible), and
notes. Blind model identity, review per-criterion discrepancies across actions
and variants, revise on development examples and preserve a held-out set.
A 30–50-response pilot is a starting workflow, not a reliability guarantee.

## Checks

```bash
gofmt -l cmd internal web
go vet ./...
go test ./...
go test -race ./...    # Requires cgo and a supported C compiler.
```

Tests cover golden prompts, schemas, protocol edge cases, HTTP fixtures/retries,
durable resume/rejudge, redaction, cohort export and offline rebuilding.
Golden updates: `UPDATE_GOLDEN=1 go test ./internal/benchmarks`.
Fixture updates: `UPDATE_FIXTURES=1 go test ./internal/runner`; review their diffs.
Local race testing was unavailable on the implementation host (cgo disabled, no
C compiler); CI is configured to run it on Linux, not claimed to have passed yet.

Optional real-browser checks need Node 22+, Chrome/Edge and the installed
design-taste-frontend skill, not a frontend dependency:

```bash
RENDER_REVIEW_DIR="$PWD/.benchmark-data/browser-demo" go test ./internal/app/sitecli -run TestUpdateWholeCohortAndOfflineBuild -count=1
TASTE_SKILL='C:/Users/ahmed/.pi/agent/skills/design-taste-frontend' node scripts/browser-check.mjs .benchmark-data/browser-demo
```

The typography-only research UI adapts tasteskill v2's marketing imagery/motion
quotas; wide/narrow synthetic views and interaction checks were reviewed. No
unqualified marketing-skill compliance or live benchmark findings are claimed.

References: [tool calling](https://openrouter.ai/docs/guides/features/tool-calling),
[Jev](https://openrouter.ai/docs/guides/community/jev),
[score/confidence](https://docs.typesafe.ai/primitives/score),
[routing](https://openrouter.ai/docs/guides/routing/provider-selection),
[model-judge limitations](https://arxiv.org/abs/2306.05685).
Scenario references S11/S12:
[PowerPyx](https://www.powerpyx.com/detroit-become-human-meet-kamski-walkthrough-100/),
[Shacknews](https://www.shacknews.com/article/105291/meet-kamski---detroit-become-human).
