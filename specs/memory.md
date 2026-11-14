# memory

## Purpose
Memory stored as claims, not prose, in SQLite, with provenance and time. Package `memory`.

## Requirements
- Claim fields: subject, predicate, object, source id (the eventlog event id it came from), source kind (user_asserted, document_derived, untrusted_external, model_inferred), confidence, valid_from, valid_to, recorded_at, status (active, superseded, contested).
- A new value for the same subject+predicate supersedes the old claim: old gets valid_to and status superseded; history stays queryable "as of" a date.
- Untrusted-external claims enter with lower confidence and status contested/unconfirmed (not active) until corroborated by a second independent source (different source id AND not the same source kind chain).
- model_inferred claims MUST NEVER count as corroboration.
- Promotion to active MUST be written to the eventlog as a memory event.
- Every memory write is logged to the eventlog.
- Uncorroborated untrusted claims MUST NOT be returned as confirmed facts by the normal query API.

## Acceptance checks
- Given "employer = A" then later "employer = B", when queried as of a date between the two, then A is returned, and B is the current value now.
- Given one uncorroborated email claim, then it is never returned as a confirmed fact.
- Given a second independent source agreeing, then it is promoted to active and the promotion is logged in the eventlog.
- Given a model_inferred claim agreeing with an untrusted claim, then the untrusted claim is NOT promoted.
- Given two claims from the same source id, then they do not corroborate each other.

## Out of scope
- Embeddings, retrieval ranking, conflict resolution UI.
