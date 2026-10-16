# Implementation plan

Specification 1.0; project started 2026-10-15. Empty repository, no inherited instructions or user changes.

## Boundaries and interfaces

Go 1.22; standard library for production. `domain.Benchmark`, consumer-owned
`runner.AgentClient`, `runner.Judge`, `runner.Store`, and `judge.DecisionsClient`.
Scenario assets embedded in family packages; explicit registration in benchmarks.
Provider DTOs isolated in providers/openrouter. CLI composition in app, never in
site. Normalized results shared across families. Static assets embedded under web.
Raw attempts are private, public exports explicit, HTML entirely offline.

## Milestones (each committed after checks)

- [x] Scaffold commands, module, ignore rules, plan and architecture.
- [ ] Versioned domain/protocol, schemas and fixtures.
- [ ] Four embedded scenario variants, validated data and golden snapshots.
- [ ] Isolated OpenRouter transports, capability discovery, fixtures/retries.
- [ ] Deterministic trial planning, atomic persistence, execution and resume.
- [ ] Blinded Jev rubric, typed normalization, independent rejudging.
- [ ] Sanitized cohort export and static comparison/trial/methodology views.
- [ ] End-to-end hardening, methodology and operator/extension documentation.

Build a fixture-backed path before the UI. Run focused tests and inspect diffs per
step; final gofmt, vet, full tests, race if available. No live inference without an
explicitly authorized small scope. No push, deployment or live-data publication.

## UI practice

Installed skill: C:/Users/ahmed/.pi/agent/skills/design-taste-frontend/SKILL.md,
tasteskill v2. Read before UI work; document adapted guidance and inspection in
docs/design.md. Preserve Go templates/minimal JS and accessible research tables
rather than adopting a frontend framework or decorative marketing animations.

## API verification

Fetched Decisions reference/tutorial, TypeSafe score, discovery and routing docs
during implementation. Reference/tutorial examples name 1.13, unlike specification
default 1.14: retain configured 1.14; discovery must fail explicitly if unavailable.
Recheck alpha docs before any first live test. See docs/sources.md.
