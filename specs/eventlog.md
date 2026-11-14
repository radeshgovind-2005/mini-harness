# eventlog

## Purpose
Append-only, hash-chained event log in SQLite (modernc.org/sqlite, no cgo). Every action (model call, tool call, policy decision, memory write) is a structured event. Package `eventlog`.

## Requirements
- Event fields: id, task id, agent id, type, payload (JSON), timestamp, previous hash, hash.
- Hash covers all other fields plus the previous hash (genesis uses a fixed zero hash).
- The system MUST detect tampering via `Verify()`, returning the first bad event's id.
- The system MUST NOT expose any update or delete path in the Go API (no Update/Delete methods; only Append, Get/List, Verify).
- Appends MUST be safe for concurrent writers (serialize in a transaction so the chain never forks).
- Timestamps injectable via a clock interface so tests are deterministic.

## Acceptance checks
- Given a log of 100 events, when one event's payload is altered directly in the database, then Verify() fails and names the first bad event.
- Given an untouched 100-event log, then Verify() passes.
- Given a deleted middle row or a rewritten hash, then Verify() fails and names the first affected event.
- A reflection/API test asserts the exported log type has no method whose name starts with Update or Delete.

## Out of scope
- Signing with keys, remote anchoring, log rotation.
