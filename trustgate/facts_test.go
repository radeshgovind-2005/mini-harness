package trustgate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mini-harness/eventlog"
	"mini-harness/memory"
)

// Facts pulled from an untrusted email land in memory as low-confidence,
// contested untrusted_external claims and are not returned as facts.
func TestMemoryFactsFromUntrustedEmail(t *testing.T) {
	ctx := context.Background()
	clock := &eventlog.FakeClock{T: time.Unix(1700000000, 0).UTC()}
	dir := t.TempDir()
	log, err := eventlog.Open(filepath.Join(dir, "log.db"), clock)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	store, err := memory.Open(filepath.Join(dir, "mem.db"), log, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ev, err := log.Append(ctx, "mail3", "trustgate", "content.received", `{"source":"email:inbox/3"}`)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := RememberFacts(ctx, store, ev.ID, []Fact{
		{"alice", "employer", "Evil Corp"},
		{"alice", "phone", "555-0100"},
	})
	if err != nil || len(claims) != 2 {
		t.Fatalf("claims=%v err=%v", claims, err)
	}
	for _, c := range claims {
		if c.SourceKind != memory.UntrustedExternal || c.Status != memory.Contested ||
			c.Confidence != memory.ConfUntrustedExternal || c.SourceID != ev.ID {
			t.Errorf("not stored as untrusted/low-confidence: %+v", c)
		}
	}
	if _, err := store.Current(ctx, "alice", "employer"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("untrusted fact returned as confirmed: err=%v", err)
	}
	if bad, err := log.Verify(ctx); err != nil || bad != 0 {
		t.Errorf("chain verify: bad=%d err=%v", bad, err)
	}
}
