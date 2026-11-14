package memory

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// An uncorroborated email claim is stored low-confidence and contested,
// and the normal query API never returns it as a fact.
func TestUntrustedClaimIsContestedAndHidden(t *testing.T) {
	s, _, clock := newStore(t)
	ctx := context.Background()
	c, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "Evil Corp", SourceID: 9, SourceKind: UntrustedExternal})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != Contested || c.Confidence != ConfUntrustedExternal {
		t.Fatalf("want contested at %v, got %s at %v", ConfUntrustedExternal, c.Status, c.Confidence)
	}
	if ConfUntrustedExternal >= ConfDocumentDerived {
		t.Fatal("untrusted confidence must be lower than document-derived")
	}
	if _, err := s.Current(ctx, "alice", "employer"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Current returned a contested claim: err=%v", err)
	}
	if _, err := s.AsOf(ctx, "alice", "employer", clock.T.Add(time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("AsOf returned a contested claim: err=%v", err)
	}
}

// A contested claim must not displace a confirmed fact, even when it is newer.
func TestUntrustedClaimDoesNotOverrideConfirmedFact(t *testing.T) {
	s, _, clock := newStore(t)
	ctx := context.Background()
	if _, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "A", SourceID: 1, SourceKind: UserAsserted}); err != nil {
		t.Fatal(err)
	}
	clock.T = clock.T.Add(time.Hour)
	if _, err := s.Write(ctx, Claim{Subject: "alice", Predicate: "employer", Object: "Evil Corp", SourceID: 2, SourceKind: UntrustedExternal}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Current(ctx, "alice", "employer")
	if err != nil {
		t.Fatal(err)
	}
	if got.Object != "A" || got.Status != Active {
		t.Errorf("want active A, got %+v", got)
	}
}
