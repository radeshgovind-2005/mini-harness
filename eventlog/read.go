package eventlog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ErrNotFound is returned by Get when no event has the given id.
var ErrNotFound = sql.ErrNoRows

const selectEvents = `SELECT id, task_id, agent_id, type, payload, ts_nanos, prev_hash, hash FROM events`

type scanner interface{ Scan(dest ...any) error }

func scanEvent(s scanner) (Event, error) {
	var e Event
	var nanos int64
	if err := s.Scan(&e.ID, &e.TaskID, &e.AgentID, &e.Type, &e.Payload, &nanos, &e.PrevHash, &e.Hash); err != nil {
		return Event{}, err
	}
	e.Timestamp = time.Unix(0, nanos).UTC()
	return e, nil
}

// Get returns the event with the given id, or ErrNotFound.
func (l *Log) Get(ctx context.Context, id int64) (Event, error) {
	return scanEvent(l.db.QueryRowContext(ctx, selectEvents+` WHERE id = ?`, id))
}

// List returns all events of a task ordered by id. An empty taskID returns every event.
func (l *Log) List(ctx context.Context, taskID string) ([]Event, error) {
	q, args := selectEvents+` ORDER BY id`, []any(nil)
	if taskID != "" {
		q, args = selectEvents+` WHERE task_id = ? ORDER BY id`, []any{taskID}
	}
	rows, err := l.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("eventlog: list: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
