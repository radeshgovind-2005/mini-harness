# decisions

Answers to planning questions. Binding on all specs.

- **Network:** allowed once for `go get modernc.org/sqlite` and `go mod tidy`. After that, tests need no network. No vendoring.
- **Model interface (fakemodel):** `Complete(ctx context.Context, msgs []Message) (Response, error)`. `Response` holds either `Text` or a list of `ToolCall{Name, Args}`. Keep it minimal.
- **Corroboration:** two claims corroborate when they agree on subject, predicate and object, have different source ids, and the corroborating claim's kind is not model_inferred. Do not build provenance-chain tracking. README Limitations must say "independence" is only approximated by distinct source ids.
- **Confidence defaults:** user_asserted 0.9, document_derived 0.7, model_inferred 0.4, untrusted_external 0.3. Use constants. A fresh unpromoted untrusted_external claim gets status `contested` (no new status). Promotion sets it `active` and raises confidence to 0.6.
- **Supersede vs untrusted:** an unpromoted untrusted_external claim never supersedes an active claim; it is stored as contested. On promotion it then supersedes.
- **Supervisor events:** dedicated event types `step.started`, `step.completed`, `step.retry`, `step.fix_the_call`, `step.escalated`. Defaults: retry cap 5 attempts, base backoff 100ms doubling, injectable sleeper.
- **Tainted args:** each action argument is `Arg{Value string, Source Provenance}` where Provenance is Trusted or Tainted, set by the caller. Outbound/destructive recipient/URL/command args with Tainted provenance are blocked even with confirm.
