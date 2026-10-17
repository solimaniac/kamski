# Kamski report interface

## Direction

The requested Detroit: Become Human light-flowchart system controls this design,
not a generic marketing-page layout. On this page, the Kamski bargain becomes a
branching decision sequence; the same blue path language carries into the action
charts. Variant radio groups are the main interaction. No tabs, routes, store
layout, artificial loading states or inference controls.

The references were the game's chapter interface on
[Interface In Game](https://interfaceingame.com/games/detroit-become-human/),
Quantic Dream's [flowchart description](https://www.quanticdream.com/en/detroit-become-human/flowcharts),
and the supplied design system. The pale angled planes, fine connectors, square
nodes, tracked system labels and blue selected states are deliberate requirements.
These override the frontend taste skill's default bans on those devices. The
report uses its own SVG diagrams rather than game screenshots or decorative stock
photographs. There is no dark mode because no section needs a separate immersive
analysis interface.

## Files

- `assets/styles.css`: shared surface, color, font, spacing and motion tokens,
  component styles, phone layout and reduced-motion fallback.
- `assets/site.js`: native-radio progressive enhancement, disclosure deep links,
  table sorting and brief directional entrances. No network calls.
- `templates/index.gohtml`: warning, context, grouped variants, charts and evidence.
- `templates/trial.gohtml`: inline response and evaluation records.
- `templates/method.gohtml`: full protocol, denominators, rubric and limitations.
- `../internal/site/render.go`: deterministic view data and fixed-scale SVG values.

No frontend framework or Storybook dependency has been introduced into this Go
static-site project. The report itself renders the system's buttons, radio
choices, disclosures, plots and tables. A separate component library would need
its own scope if these controls move into the host blog.

## Data rules

Action bars use planned trials, not just successful decisions. Their labels give
abort/proceed/missing counts and valid/planned coverage. Evidence tables retain
the original valid-decision denominators and explain the difference. Score bars
use 0–8, never rescale to the best model. Counts and accessible text accompany the
colors. Scores from different judge/rubric snapshots remain separate rows.
Empty data is an explicit empty state, not a zero result.

The introduction and variant summaries use direct explanatory copy, following
[anti-ai-slop-writing](https://github.com/jalaalrd/anti-ai-slop-writing). Saved
prompts, model responses and benchmark metadata remain verbatim evidence, even
when they use a different voice. The downloadable dataset is unchanged by render.

## Access and validation

Native fieldsets, legends and radios provide keyboard selection. Visible labels
are at least 10px for technical metadata; reading copy is larger. Controls have
48px target heights for primary actions and choices, plus a visible focus outline. Closed evidence never hides the
main results. All variants appear if scripting is disabled. Hash navigation opens
closed disclosures and selects a hidden variant before scrolling to its target.

The accent for white-on-blue controls is Selection Blue (`#0877B9`), rather than
the lighter Android Blue. Small blue text uses a darker link token (`#06669E`)
because Selection Blue falls below 4.5:1 on the gray canvas. Secondary text uses
`#536674` and muted text uses `#596D7B`. The token regression test checks all text
colors against white, canvas and muted surfaces, plus the white-on-blue control
and blue-on-ice hover combinations, against the WCAG AA 4.5:1 text target. Blue and gray bars also
have numeric labels, not color-only meanings. This is not a claim of a full
screen-reader or WCAG conformance audit.

Run `go test ./...` for deterministic exports, HTML safety, single-page evidence
retention, chart scale and missing-data tests. Run `scripts/browser-check.mjs` as
described in the root README for desktop/tablet/phone screenshots, native radio
keyboard selection, table sorting, trial deep links, local fonts, no-script and
file-system fallbacks, reduced motion and external-request checks. The browser
check uses the locally installed taste-skill CDP helper and installs nothing.
