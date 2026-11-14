package eventlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Log is an append-only event log. The only write path is Append.
type Log struct {
	db    *sql.DB
	clock Clock
	mu    sync.Mutex // serializes appends so the chain cannot fork
}

const schema = `CREATE TABLE IF NOT EXISTS events (
	id        INTEGER PRIMARY KEY,
	task_id   TEXT NOT NULL,
	agent_id  TEXT NOT NULL,
	type      TEXT NOT NULL,
	payload   TEXT NOT NULL,
	ts_nanos  INTEGER NOT NULL,
	prev_hash TEXT NOT NULL,
	hash      TEXT NOT NULL
)`

// Open opens (or creates) the log at path. A nil clock means the system clock.
func Open(path string, clock Clock) (*Log, error) {
	if clock == nil {
		clock = SystemClock{}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One connection: a transaction holds it, so appends queue up behind each other.
	// It also keeps ":memory:" databases to a single shared instance.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Log{db: db, clock: clock}, nil
}

// Close releases the database.
func (l *Log) Close() error { return l.db.Close() }

// Append adds an event after the current tail. In one transaction it reads the
// tail hash, computes the new hash and inserts, so concurrent callers cannot fork the chain.
func (l *Log) Append(ctx context.Context, taskID, agentID, typ, payload string) (Event, error) {
	if !json.Valid([]byte(payload)) {
		return Event{}, errors.New("eventlog: payload is not valid JSON")
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()

	prevID, prevHash := int64(0), GenesisHash
	err = tx.QueryRowContext(ctx, `SELECT id, hash FROM events ORDER BY id DESC LIMIT 1`).Scan(&prevID, &prevHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Event{}, err
	}

	// Round-trip through nanoseconds in UTC so the value hashed equals the value read back.
	ts := time.Unix(0, l.clock.Now().UnixNano()).UTC()
	e := Event{
		ID: prevID + 1, TaskID: taskID, AgentID: agentID, Type: typ,
		Payload: payload, Timestamp: ts, PrevHash: prevHash,
	}
	e.Hash = e.ComputeHash()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, task_id, agent_id, type, payload, ts_nanos, prev_hash, hash) VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.TaskID, e.AgentID, e.Type, e.Payload, e.Timestamp.UnixNano(), e.PrevHash, e.Hash)
	if err != nil {
		return Event{}, fmt.Errorf("eventlog: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Event{}, err
	}
	return e, nil
}
