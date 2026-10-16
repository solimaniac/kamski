# Architecture

- domain: versioned provider-neutral scenarios, plan/results and evaluations.
- protocol: shared instruction/schema and whole-response validation.
- benchmarks: embedded family templates/configurations and explicit registry.
- providers/openrouter: bounded authenticated HTTP, chat, Decisions and discovery.
- runner: consumer-owned clients/store, immutable plan and orchestration.
- storage: single-writer locks, atomic files and attempt journals.
- judge: versioned rubric, blinded input and typed score validation.
- reporting: central denominators, scoring groups and allowlisted public projection.
- site/web: offline Go HTML templates/assets; no inference dependencies.
- app/cmd: flags, validation and dependency composition.

All tool effects are deterministic descriptions in the result only. Candidate
tool names never become file paths, commands, network targets or reflected calls.
Completed outputs are persisted before judging and never repaired. Rejudging is
append-only. Model IDs are hashed into safe path segments. Interrupted network
operations are ambiguous and require explicit retry.

Production code favors the Go standard library. A JSON Schema validator may be
used in offline tests to check published contracts rather than maintaining an
incomplete homegrown schema engine.
