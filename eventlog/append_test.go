package eventlog

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTemp(t *testing.T, clock Clock) *Log {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "log.db"), clock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestAppendChainsHashes(t *testing.T) {
	clock := &FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := openTemp(t, clock)
	ctx := context.Background()

	first, err := l.Append(ctx, "t1", "a1", "tool.call", `{"n":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != 1 || first.PrevHash != GenesisHash {
		t.Fatalf("first event should be id 1 on genesis, got id %d prev %s", first.ID, first.PrevHash)
	}
	clock.Advance(time.Second)
	second, err := l.Append(ctx, "t1", "a1", "tool.result", `{"n":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != 2 || second.PrevHash != first.Hash {
		t.Fatalf("second event must chain to first: %+v", second)
	}
	if second.Hash != second.ComputeHash() {
		t.Fatal("stored hash must match recomputed hash")
	}
}

func TestAppendRejectsInvalidJSON(t *testing.T) {
	l := openTemp(t, nil)
	if _, err := l.Append(context.Background(), "t", "a", "x", `{not json`); err == nil {
		t.Fatal("want error for invalid JSON payload")
	}
}

func TestAppendPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.db")
	ctx := context.Background()
	l, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	e1, _ := l.Append(ctx, "t", "a", "x", `{}`)
	l.Close()

	l, err = Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	e2, err := l.Append(ctx, "t", "a", "x", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if e2.ID != 2 || e2.PrevHash != e1.Hash {
		t.Fatalf("reopened log must continue the chain: %+v", e2)
	}
}
