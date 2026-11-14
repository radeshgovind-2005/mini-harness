package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mini-harness/eventlog"

	_ "modernc.org/sqlite"
)

// Store keeps claims in SQLite and logs every write to the event log.
type Store struct {
	db    *sql.DB
	log   *eventlog.Log
	clock eventlog.Clock
}

const schema = `CREATE TABLE IF NOT EXISTS claims (
	id          INTEGER PRIMARY KEY,
	subject     TEXT NOT NULL,
	predicate   TEXT NOT NULL,
	object      TEXT NOT NULL,
	source_id   INTEGER NOT NULL,
	source_kind TEXT NOT NULL,
	confidence  REAL NOT NULL,
	valid_from  INTEGER NOT NULL,
	valid_to    INTEGER,           -- NULL = still open
	recorded_at INTEGER NOT NULL,
	status      TEXT NOT NULL
)`

// Open opens (or creates) the claim store at path. A nil clock means the system clock.
func Open(path string, log *eventlog.Log, clock eventlog.Clock) (*Store, error) {
	if log == nil {
		return nil, errors.New("memory: event log is required")
	}
	if clock == nil {
		clock = eventlog.SystemClock{}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, log: log, clock: clock}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// Write records a new claim. Confidence and status come from the source kind:
// untrusted_external claims start contested, everything else starts active.
// Caller sets Subject, Predicate, Object, SourceID, SourceKind and optionally ValidFrom
// (default: now). The write is logged as a "memory.write" event.
func (s *Store) Write(ctx context.Context, c Claim) (Claim, error) {
	conf, ok := defaultConfidence(c.SourceKind)
	if !ok {
		return Claim{}, fmt.Errorf("memory: unknown source kind %q", c.SourceKind)
	}
	if c.Subject == "" || c.Predicate == "" {
		return Claim{}, errors.New("memory: subject and predicate are required")
	}
	now := s.clock.Now().UTC()
	c.Confidence = conf
	c.RecordedAt = now
	c.ValidTo = time.Time{}
	if c.ValidFrom.IsZero() {
		c.ValidFrom = now
	}
	c.ValidFrom = c.ValidFrom.UTC()
	c.Status = Active
	if c.SourceKind == UntrustedExternal {
		c.Status = Contested
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO claims (subject, predicate, object, source_id, source_kind, confidence, valid_from, recorded_at, status)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		c.Subject, c.Predicate, c.Object, c.SourceID, string(c.SourceKind), c.Confidence,
		c.ValidFrom.UnixNano(), c.RecordedAt.UnixNano(), string(c.Status))
	if err != nil {
		return Claim{}, fmt.Errorf("memory: insert: %w", err)
	}
	if c.ID, err = res.LastInsertId(); err != nil {
		return Claim{}, err
	}
	if err := s.logEvent(ctx, "memory.write", c); err != nil {
		return Claim{}, err
	}
	return c, nil
}

// logEvent appends a memory event describing the claim.
func (s *Store) logEvent(ctx context.Context, typ string, c Claim) error {
	payload, err := json.Marshal(map[string]any{
		"claim_id": c.ID, "subject": c.Subject, "predicate": c.Predicate, "object": c.Object,
		"source_id": c.SourceID, "source_kind": c.SourceKind, "confidence": c.Confidence,
		"status": c.Status,
	})
	if err != nil {
		return err
	}
	_, err = s.log.Append(ctx, "memory", "memory", typ, string(payload))
	return err
}

// Get returns a claim by id, whatever its status.
func (s *Store) Get(ctx context.Context, id int64) (Claim, error) {
	var c Claim
	var kind, status string
	var from, rec int64
	var to sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, subject, predicate, object, source_id, source_kind, confidence, valid_from, valid_to, recorded_at, status
		 FROM claims WHERE id = ?`, id).
		Scan(&c.ID, &c.Subject, &c.Predicate, &c.Object, &c.SourceID, &kind, &c.Confidence, &from, &to, &rec, &status)
	if err != nil {
		return Claim{}, err
	}
	c.SourceKind, c.Status = SourceKind(kind), Status(status)
	c.ValidFrom, c.RecordedAt = time.Unix(0, from).UTC(), time.Unix(0, rec).UTC()
	if to.Valid {
		c.ValidTo = time.Unix(0, to.Int64).UTC()
	}
	return c, nil
}
