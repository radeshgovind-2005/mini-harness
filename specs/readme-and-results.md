# readme-and-results

## Purpose
The README is what the reader sees first. Clarity and honesty over features.

## Requirements
- Short architecture diagram (ASCII or mermaid) showing eventlog, supervisor, memory, trustgate, fakemodel.
- "Results" section at the top with ACTUAL measured numbers copied from real test runs (crash test runs/passes, injection test outcome, supersede test outcome, chain verify outcome). Never invent numbers.
- "Limitations" section: states the screen and policy layer are a layer of defense, not a guarantee; heuristics are bypassable; single-node SQLite; hash chain detects tampering but not a rewriter who recomputes the whole chain; fake model only; etc.
- Go 1.22+, single module, minimal deps (modernc.org/sqlite only).
- Optional LAST item, only after all four topics pass: a real local model adapter behind the same model interface.

## Acceptance checks
- Given the repo, `go vet ./... && go test ./...` passes.
- Given the README, it contains "Results" and "Limitations" headings, and the Results numbers match a fresh `go test -v` run.

## Out of scope
- UI, multi-user, real provider routing, sandboxing arbitrary code.
