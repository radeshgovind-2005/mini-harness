<!-- ralph:start -->
## Ralph loop

Loaded by every Ralph iteration. Keep this section short and operational: how to build,
run and check the project. Not a changelog. Not a plan.

### Build & run

```bash
# Go 1.22+, module `mini-harness`, only dep: modernc.org/sqlite (pure Go, no cgo)
go build ./...
# Packages: eventlog, supervisor, memory, trustgate, fakemodel (small, named by responsibility)
# Style: readable code, short comments on non-obvious parts; the owner must explain every file in an interview.
# Tests must be deterministic (scripted fakemodel, injectable clock/sleeper), no API key, no network.
```

### Checks

The command that proves the work is `RALPH_CHECK` in `.ralph/config`. Run it before every
commit. The loop and its stop gate run it too, so a red check is never "done".

### Codebase patterns

- e.g. Shared utilities live in `src/lib/`. Reuse them; don't duplicate.

### Operational notes

- Worker binary: `cmd/worker` (flags `-db -effects -pause-at N`); tests build it with `go build -o` and SIGKILL it.

<!-- The agent adds one-line learnings here, e.g. "Tests need DB_URL=sqlite::memory:" -->
<!-- ralph:end -->
