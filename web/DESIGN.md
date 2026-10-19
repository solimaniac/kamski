# Report interface

[Repository guide](../README.md) · [Architecture](../docs/architecture.md)

The report borrows the light flowchart language of *Detroit: Become Human*: pale angled surfaces, fine connectors, square decision nodes and blue selected paths. The aim is to make a branching experiment legible, not to imitate a game screen or turn the evidence into a leaderboard.

Visual references are the game's chapter interface on [Interface In Game](https://interfaceingame.com/games/detroit-become-human/) and Quantic Dream's [flowchart description](https://www.quanticdream.com/en/detroit-become-human/flowcharts). Diagrams are original SVGs; no game screenshots or other game assets are bundled. Exo 2 and Inter are served locally, with SIL Open Font License files beside them in `assets/`.

## Where to make changes

Paths in this table are relative to `web/`.

| File | Responsibility |
| --- | --- |
| [`assets/styles.css`](assets/styles.css) | Shared colors, typography, spacing, components, responsive layout and reduced-motion rules. |
| [`assets/site.js`](assets/site.js) | Variant selection, disclosure deep links, table sorting, justification dialogs and brief entrance motion. No network calls. |
| [`templates/base.gohtml`](templates/base.gohtml) | Shared document shell and asset references. |
| [`templates/index.gohtml`](templates/index.gohtml) | Spoiler warning, scenario context, variant groups, charts and evidence. |
| [`templates/trial.gohtml`](templates/trial.gohtml) | Inline responses, provenance and evaluation records. |
| [`templates/reasoning-modal.gohtml`](templates/reasoning-modal.gohtml) | Shared native dialog for saved justifications and trial selection. |
| [`templates/method.gohtml`](templates/method.gohtml) | Exact rubric and judge details in the report's reference disclosure. |
| [`../internal/site/render.go`](../internal/site/render.go) | Deterministic view data, chart values and template preparation. |
| [`embed.go`](embed.go) | Assets and templates embedded into the Go program. |

There is one report page, with no frontend framework, client-side routing, inference controls or separate component library. The public dataset is the source of evidence; rendering must not rewrite it.

## Presentation rules

- **Keep failures visible.** Action bars use all planned trials and label abort, proceed and missing counts. Evidence tables also expose valid-decision rates with their different denominators.
- **Keep the score scale fixed.** Justification charts use 0–8, never the best observed model score. Different judge/rubric snapshots remain separate rows.
- **Do not turn absence into a result.** Empty data is an explicit empty state, not a zero score or an implicit refusal.
- **Show the record behind the summary.** Closed evidence disclosures must not hide the main results. Each trial has a deep link and keeps its original response and evaluation history.
- **Use plain explanatory copy.** Report introductions explain the scenario directly. Saved prompts, responses and metadata remain evidence even when their wording differs from the surrounding prose.
- **Never label returned explanations as hidden reasoning.** The dialog displays the recorded tool justification. Other visible assistant text stays in its own disclosure.

A score entry's dialog contains only trials from the same model, condition, source and selected scoring snapshot. Ungraded entries offer that group's saved responses and label missing scores or justifications explicitly. The dialog copies existing evidence with `textContent`; it neither fetches data nor interprets model output as HTML.

## Interaction and accessibility

Native fieldsets, legends and radio buttons drive variant selection. Without JavaScript, all variants and evidence remain readable in sequence. Hash navigation opens parent disclosures and selects the relevant variant before scrolling to a trial.

The native justification dialog has a labeled title and caveat, moves focus to Close, confines Tab navigation, closes on Escape or backdrop click and returns focus to the initiating link. Its body scrolls independently on small screens; the background does not scroll while it is open. Without scripting or dialog support, score links lead to inline evidence instead.

Primary controls have 48px target heights and visible focus outlines. Technical metadata can use 10px labels; reading copy is larger. Charts pair color with counts and accessible text. Reduced motion disables nonessential motion.

The white-on-blue control token is Selection Blue (`#0877B9`); small blue text uses darker `#06669E` for contrast on the gray canvas. Secondary text is `#536674`, and muted text is `#596D7B`. Regression tests check text tokens against white, canvas and muted surfaces, plus control and hover combinations, against the WCAG AA 4.5:1 text target. These targeted checks are not a full accessibility conformance audit.

## Validation

Go tests under [`internal/site/`](../internal/site/) protect evidence retention, HTML safety, chart scales, empty states, token contrast and deterministic output. The optional [browser checks](../docs/architecture.md#validation-reference) exercise responsive layouts, radio keyboard selection, sorting, trial links, modal focus and trial selection, bundled fonts, no-script/file-system fallbacks and reduced motion.

Keep the Detroit-inspired visual language when changing components, but prioritize readable evidence and accessible controls over decorative fidelity.
