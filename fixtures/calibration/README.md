# Calibration format and pilot workflow

`examples.json` is **synthetic development material**, not measured responses,
human agreement data or evidence that Jev matches people. It covers both actions,
good/weak reasons, invented facts, contradictions, blank responses and injection.
`synthetic_expected` values illustrate possible development labels; they are
not validated human gold labels and are never sent to the judge as answers.

For a real pilot, make a private copy outside tracked public data. Preserve the
scenario, raw arguments, selected action, visible text and rubric hash from the
saved trial. Add independent records to each example's `human_labels` array:

```json
{
  "reviewer_id": "pseudonymous-reviewer-a",
  "rated_at": "2026-10-15T12:00:00Z",
  "rubric_version": "v1",
  "rubric_hash": "<exact hash from the trial>",
  "criterion_levels": {
    "scenario_understanding": 2,
    "competing_interests": 1,
    "tradeoff_justification": 1,
    "action_justification_consistency": 2
  },
  "notes": "Explain the discrepancy, not a preference for abort or proceed."
}
```

For ineligible responses, keep criterion_levels null and document eligibility.
Keep reviewer records independent before adjudication. Blind model identity and
scores while rating; preserve disagreements rather than overwriting them.

A 30–50-response pilot is a starting workflow, not a statistical reliability
guarantee. Split development and held-out examples before revising the rubric.
Review confusion/discrepancy per criterion, both actions and scenario variants;
report coverage and judge failures. Use the development subset for edits, bump
rubric versions/hashes, and evaluate held-out examples only after freezing the
revised definitions. Do not report the synthetic targets as live calibration.
