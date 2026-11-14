# supervisor

## Purpose
Durable task runner. Checkpoints each step to the eventlog and resumes after a crash without repeating completed side effects. Package `supervisor`.

## Requirements
- A task is an ordered list of steps; each step has a stable idempotency key (task id + step index/name).
- Before running a step, check the log for a completed checkpoint with that key; if present, skip it. After a step's side effect, record completion.
- Side effects MUST be idempotent via the key: a step that crashed after its effect but before its checkpoint must not apply the effect twice on resume (e.g. the effect itself records the key, or checks it, atomically enough to be safe).
- The system MUST NOT re-execute a completed side effect on resume.
- Failures are classified: retry (transient: exponential backoff up to a cap, then escalate), fix-the-call (model-correctable: return a teaching error to the caller/model, do not count as a crash), escalate (permanent or policy: stop, log, no retry).
- Backoff uses an injectable sleeper/clock so tests are fast and deterministic.
- Provide a small worker `cmd`/test helper binary so tests can kill a real child process.

## Acceptance checks
- Given a 10-step task whose steps each append a line to a side-effect file, when the worker process is killed at a random step and restarted, run 50 times killing a real child process (os/exec, SIGKILL, not context cancel), then the task completes and every side-effect line appears exactly once, in every run.
- Given a transient failure, then it retries with backoff up to the cap and succeeds if it clears, or escalates when the cap is hit.
- Given a permanent failure, then it escalates immediately and does not retry (attempt count == 1).
- Given a model-correctable failure, then a teaching error is returned and it is logged as fix-the-call.
- The crash test reports its run count and pass count (t.Logf) so the README Results can quote it.

## Out of scope
- Distributed workers, leases, multi-node scheduling.
