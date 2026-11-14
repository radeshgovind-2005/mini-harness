package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mini-harness/eventlog"
)

func newStore(t *testing.T) (*Store, *eventlog.Log, *eventlog.FakeClock) {
	t.Helper()
	clock := &eventlog.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	log, err := eventlog.Open(filepath.Join(dir, "log.db"), clock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	s, err := Open(filepath.Join(dir, "mem.db"), log, clock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, log, clock
}

func TestWriteDefaultsByKind(t *testing.T) {
	s, _, clock := newStore(t)
	ctx := context.Background()
	cases := []struct {
		kind   SourceKind
		conf   float64
		status Status
	}{
		{UserAsserted, ConfUserAsserted, Active},
		{DocumentDerived, ConfDocumentDerived, Active},
		{ModelInferred, ConfModelInferred, Active},
		{UntrustedExternal, ConfUntrustedExternal, Contested},
	}
	for i, tc := range cases {
		c, err := s.Write(ctx, Claim{Subject: "alice", Predicate: string(tc.kind), Object: "x", SourceID: int64(i + 1), SourceKind: tc.kind})
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Confidence != tc.conf || got.Status != tc.status {
			t.Errorf("%s: got conf %v status %s, want %v %s", tc.kind, got.Confidence, got.Status, tc.conf, tc.status)
		}
		if !got.RecordedAt.Equal(clock.T) || !got.ValidFrom.Equal(clock.T) || !got.ValidTo.IsZero() {
			t.Errorf("%s: unexpected times %+v", tc.kind, got)
		}
	}
}

func TestWriteIsLoggedAndChainVerifies(t *testing.T) {
	s, log, _ := newStore(t)
	ctx := context.Background()
	c, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "A", SourceID: 7, SourceKind: UserAsserted})
	if err != nil {
		t.Fatal(err)
	}
	evs, err := log.List(ctx, "memory")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Type != "memory.write" {
		t.Fatalf("want one memory.write event, got %+v", evs)
	}
	if c.ID != 1 {
		t.Fatalf("claim id = %d", c.ID)
	}
	if bad, err := log.Verify(ctx); err != nil || bad != 0 {
		t.Fatalf("verify: bad=%d err=%v", bad, err)
	}
}

func TestWriteRejectsBadInput(t *testing.T) {
	s, log, _ := newStore(t)
	ctx := context.Background()
	if _, err := s.Write(ctx, Claim{Subject: "a", Predicate: "p", SourceKind: "bogus"}); err == nil {
		t.Error("unknown source kind should fail")
	}
	if _, err := s.Write(ctx, Claim{Predicate: "p", SourceKind: UserAsserted}); err == nil {
		t.Error("empty subject should fail")
	}
	if evs, _ := log.List(ctx, "memory"); len(evs) != 0 {
		t.Errorf("failed writes must not be logged, got %d events", len(evs))
	}
}
