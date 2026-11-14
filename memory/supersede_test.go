package memory

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestSupersedeEmployerAThenB(t *testing.T) {
	s, log, clock := newStore(t)
	ctx := context.Background()
	t1 := clock.T
	a, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "A", SourceID: 1, SourceKind: UserAsserted})
	if err != nil {
		t.Fatal(err)
	}
	clock.T = t1.Add(48 * time.Hour)
	t2 := clock.T
	b, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "B", SourceID: 2, SourceKind: UserAsserted})
	if err != nil {
		t.Fatal(err)
	}

	old, _ := s.Get(ctx, a.ID)
	if old.Status != Superseded || !old.ValidTo.Equal(t2) {
		t.Errorf("old claim not closed: %+v", old)
	}
	if b.Status != Active {
		t.Errorf("new claim status = %s", b.Status)
	}

	between, err := s.AsOf(ctx, "alice", "employer", t1.Add(24*time.Hour))
	if err != nil || between.Object != "A" {
		t.Errorf("as of between = %+v, %v; want A", between, err)
	}
	now, err := s.Current(ctx, "alice", "employer")
	if err != nil || now.Object != "B" {
		t.Errorf("current = %+v, %v; want B", now, err)
	}
	if _, err := s.AsOf(ctx, "alice", "employer", t1.Add(-time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("before any claim: err = %v, want ErrNoRows", err)
	}

	evs, _ := log.List(ctx, "memory")
	var supersedes int
	for _, e := range evs {
		if e.Type == "memory.supersede" {
			supersedes++
		}
	}
	if supersedes != 1 {
		t.Errorf("supersede events = %d, want 1", supersedes)
	}
	if bad, err := log.Verify(ctx); err != nil || bad != 0 {
		t.Fatalf("verify: bad=%d err=%v", bad, err)
	}
}

func TestContestedClaimDoesNotSupersede(t *testing.T) {
	s, _, clock := newStore(t)
	ctx := context.Background()
	a, _ := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "A", SourceID: 1, SourceKind: UserAsserted})
	clock.T = clock.T.Add(time.Hour)
	if _, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "evil", SourceID: 2, SourceKind: UntrustedExternal}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, a.ID)
	if got.Status != Active {
		t.Errorf("trusted claim status = %s, want active", got.Status)
	}
	if cur, _ := s.Current(ctx, "alice", "employer"); cur.Object != "A" {
		t.Errorf("current = %q, want A", cur.Object)
	}
}
