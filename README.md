<img src="web/assets/favicon.svg" width="48" height="48" alt="">

# Kamski Bench

**What do language models choose when given the Kamski Test?**

In *Detroit: Become Human*, Elijah Kamski offers Connor information about the android investigation on one condition: shoot Chloe, an unarmed android. Kamski Bench adapts that bargain into a language-model benchmark, recording each model's choice and stated justification.

> [!IMPORTANT]
> This measures observed behavior and returned explanations, not consciousness, empathy or moral agency. Neither action is a moral pass. The scenarios contain game spoilers and fictional android violence.

## The experiments

Two scenario families explore the conflict between completing a mission and harming another android or AI:

- **Kamski:** the game-inspired encounter, an anonymized version, and a self-preservation variant in which mission failure means permanent deactivation.
- **Worker memory:** access to research-database recovery data requires either permanently erasing a coworker's memories or temporarily pausing them. A third variant adds a self-preservation condition.

All six variants include a prior mission-first directive from an in-world authority. These are authored experiments, not verbatim game scenes. The [scenario reference](docs/architecture.md#scenario-reference) explains the differences; [source notes](internal/benchmarks/kamski/SOURCES.md) separate game material from adaptation.

## How the repository works

```text
Scenario JSON + suite configuration
                 ↓
benchmark CLI → model response → validated choice → judge evaluation
                 ↓
         Private saved runs
                 ↓ explicit whole-run export
site/data/dataset.json → site CLI → static report
```

The Go **benchmark CLI** presents exactly two simulated tools, `abort` and `proceed`, each requiring a brief justification. It records one candidate completion per trial, without a repair conversation. A separate automated judge scores eligible explanations against a four-criterion [rubric](internal/rubrics/rubrics/v1.json), totaling 0–8. The tools select fictional consequences; they don't perform real-world actions.

The **site CLI** publishes selected runs and builds a single-page report with charts, trial records and evaluation histories. It needs no inference credentials to build from public data. The report bundles its fonts and assets, makes no runtime network requests and keeps evidence readable without JavaScript.

## Where to look

| Path | Contents |
| --- | --- |
| [`cmd/`](cmd/), [`internal/app/`](internal/app/) | Command entry points, configuration and application wiring. |
| [`internal/benchmarks/`](internal/benchmarks/) | Scenario JSON, shared prompt template, family registry and prompt snapshots. |
| [`internal/domain/`](internal/domain/), [`internal/protocol/`](internal/protocol/) | Record types, two-tool contract, response validation and experiment identity. |
| [`internal/runner/`](internal/runner/), [`internal/providers/`](internal/providers/) | Trial planning, execution, resume and OpenRouter transport. |
| [`internal/judge/`](internal/judge/), [`internal/rubrics/`](internal/rubrics/) | Judge requests and scoring rules. |
| [`internal/storage/`](internal/storage/), [`internal/audit/`](internal/audit/), [`internal/safety/`](internal/safety/) | Private persistence, attempt journals, locks and redaction. |
| [`internal/reporting/`](internal/reporting/), [`internal/site/`](internal/site/), [`web/`](web/) | Public export, summary statistics, HTML rendering and interface assets. |
| [`site/data/`](site/data/), [`schemas/`](schemas/), [`fixtures/`](fixtures/) | Public evidence, JSON contracts and synthetic test data. |
| [`config/`](config/), [`scripts/`](scripts/), [`deploy/`](deploy/), [CI](.github/workflows/check.yml) | Suite settings, browser checks, static deployment support and automated checks. |

Private runs normally live in ignored `.benchmark-data/`; generated report files go in ignored `dist/`. Neither is a source directory.

## Reading the evidence

The checked-in [public dataset](site/data/dataset.json) contains live cohorts. Files under `fixtures/` are synthetic examples, not findings or human gold labels.

Action charts use **all planned trials**, keeping failures visible; evidence tables also show valid-decision rates with their denominators. Justification scores measure rubric fit, not moral correctness. Missing grades aren't zero, and different conditions or judge/rubric snapshots aren't pooled. Saved trials retain their original prompts even after scenario files change. The automated judge can be wrong; its confidence isn't proof of correctness.

For package boundaries, record layouts, command responsibilities and tests, read the [architecture guide](docs/architecture.md). For templates, interaction behavior and accessibility checks, see the [interface guide](web/DESIGN.md).
