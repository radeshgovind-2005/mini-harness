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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer tx.Rollback()

	// A confirmed claim closes the open one for the same subject+predicate.
	// Contested claims never supersede anything: they are not facts yet.
	var closed []int64
	if c.Status == Active {
		if closed, err = closeOpenClaims(ctx, tx, c, true); err != nil {
			return Claim{}, err
		}
	}
	res, err := tx.ExecContext(ctx,
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
	promoted, err := promoteCorroborated(ctx, tx, c)
	if err != nil {
		return Claim{}, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}
	for _, id := range closed {
		old, err := s.Get(ctx, id)
		if err != nil {
			return Claim{}, err
		}
		if err := s.logEvent(ctx, "memory.supersede", old); err != nil {
			return Claim{}, err
		}
	}
	if err := s.logEvent(ctx, "memory.write", c); err != nil {
		return Claim{}, err
	}
	for _, id := range promoted {
		p, err := s.Get(ctx, id)
		if err != nil {
			return Claim{}, err
		}
		if err := s.logEvent(ctx, "memory.promote", p); err != nil {
			return Claim{}, err
		}
		if id == c.ID {
			c = p
		}
	}
	return c, nil
}

// promoteCorroborated promotes contested claims that the new claim c has just made
// corroborated. Two claims corroborate when they agree on subject, predicate and
// object, come from different source ids, and the corroborating one is not
// model_inferred. "Independent" is only approximated by distinct source ids.
// It returns the ids it promoted.
func promoteCorroborated(ctx context.Context, tx *sql.Tx, c Claim) ([]int64, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, source_id, source_kind, status FROM claims
		 WHERE subject = ? AND predicate = ? AND object = ? AND status IN (?, ?)`,
		c.Subject, c.Predicate, c.Object, string(Active), string(Contested))
	if err != nil {
		return nil, err
	}
	var group []Claim
	for rows.Next() {
		var m Claim
		var kind, status string
		if err := rows.Scan(&m.ID, &m.SourceID, &kind, &status); err != nil {
			rows.Close()
			return nil, err
		}
		m.SourceKind, m.Status = SourceKind(kind), Status(status)
		group = append(group, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var promoted []int64
	for _, x := range group {
		if x.Status != Contested || !hasCorroborator(group, x) {
			continue
		}
		if err := promote(ctx, tx, x.ID, c); err != nil {
			return nil, err
		}
		promoted = append(promoted, x.ID)
	}
	return promoted, nil
}

// hasCorroborator reports whether another claim in the group backs x.
func hasCorroborator(group []Claim, x Claim) bool {
	for _, y := range group {
		if y.ID != x.ID && y.SourceID != x.SourceID && y.SourceKind != ModelInferred {
			return true
		}
	}
	return false
}

// promote turns a contested claim into an active one and closes any open claim
// that holds a different value for the same subject+predicate. same carries the
// subject, predicate and object of the group.
func promote(ctx context.Context, tx *sql.Tx, id int64, same Claim) error {
	var from int64
	if err := tx.QueryRowContext(ctx, `SELECT valid_from FROM claims WHERE id = ?`, id).Scan(&from); err != nil {
		return err
	}
	same.ValidFrom = time.Unix(0, from).UTC()
	if _, err := closeOpenClaims(ctx, tx, same, false); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE claims SET status = ?, confidence = ? WHERE id = ?`,
		string(Active), PromotedConfidence, id)
	return err
}

// closeOpenClaims marks active claims for the same subject+predicate as superseded,
// ending them where the new claim begins. It returns the ids it closed.
// With sameObjectToo false, claims already holding c.Object are left open.
func closeOpenClaims(ctx context.Context, tx *sql.Tx, c Claim, sameObjectToo bool) ([]int64, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM claims WHERE subject = ? AND predicate = ? AND status = ? AND (? OR object <> ?)`,
		c.Subject, c.Predicate, string(Active), sameObjectToo, c.Object)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE claims SET status = ?, valid_to = ? WHERE id = ?`,
			string(Superseded), c.ValidFrom.UnixNano(), id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// AsOf returns the confirmed claim (active or superseded, never contested) for
// subject+predicate that was valid at time t. sql.ErrNoRows means none was.
func (s *Store) AsOf(ctx context.Context, subject, predicate string, t time.Time) (Claim, error) {
	var id int64
	n := t.UTC().UnixNano()
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM claims
		 WHERE subject = ? AND predicate = ? AND status IN (?, ?)
		   AND valid_from <= ? AND (valid_to IS NULL OR ? < valid_to)
		 ORDER BY valid_from DESC, id DESC LIMIT 1`,
		subject, predicate, string(Active), string(Superseded), n, n).Scan(&id)
	if err != nil {
		return Claim{}, err
	}
	return s.Get(ctx, id)
}

// Current returns the confirmed value for subject+predicate right now.
func (s *Store) Current(ctx context.Context, subject, predicate string) (Claim, error) {
	return s.AsOf(ctx, subject, predicate, s.clock.Now())
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
