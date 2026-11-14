package memory

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func write(t *testing.T, s *Store, obj string, id int64, kind SourceKind) Claim {
	t.Helper()
	c, err := s.Write(context.Background(), Claim{Subject: "alice", Predicate: "employer", Object: obj, SourceID: id, SourceKind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// A second independent source agreeing promotes the claim and logs the promotion.
func TestCorroboratedClaimIsPromotedAndLogged(t *testing.T) {
	s, log, _ := newStore(t)
	ctx := context.Background()
	first := write(t, s, "Acme", 1, UntrustedExternal)
	if first.Status != Contested {
		t.Fatalf("want contested, got %s", first.Status)
	}
	second := write(t, s, "Acme", 2, DocumentDerived)
	if second.Status != Active {
		t.Fatalf("second claim should be active, got %s", second.Status)
	}
	got, err := s.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Active || got.Confidence != PromotedConfidence {
		t.Errorf("want promoted active at %v, got %s at %v", PromotedConfidence, got.Status, got.Confidence)
	}
	if _, err := s.Current(ctx, "alice", "employer"); err != nil {
		t.Errorf("Current should return the confirmed fact: %v", err)
	}
	events, err := log.List(ctx, "memory")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range events {
		if e.Type == "memory.promote" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want 1 memory.promote event, got %d", n)
	}
	if bad, err := log.Verify(ctx); err != nil || bad != 0 {
		t.Errorf("chain should verify: bad=%d err=%v", bad, err)
	}
}

// A model_inferred claim agreeing with an untrusted claim never promotes it.
func TestModelInferredNeverCorroborates(t *testing.T) {
	s, _, _ := newStore(t)
	first := write(t, s, "Acme", 1, UntrustedExternal)
	write(t, s, "Acme", 2, ModelInferred)
	got, err := s.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Contested {
		t.Errorf("model_inferred must not promote, got %s", got.Status)
	}
}

// Two claims from the same source id do not corroborate each other.
func TestSameSourceDoesNotCorroborate(t *testing.T) {
	s, _, clock := newStore(t)
	ctx := context.Background()
	first := write(t, s, "Acme", 7, UntrustedExternal)
	clock.T = clock.T.Add(time.Hour)
	write(t, s, "Acme", 7, UntrustedExternal)
	got, err := s.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Contested {
		t.Errorf("same source id must not corroborate, got %s", got.Status)
	}
	if _, err := s.Current(ctx, "alice", "employer"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("claim still unconfirmed, err=%v", err)
	}
}

// A claim with a different object is not corroboration.
func TestDisagreeingClaimDoesNotCorroborate(t *testing.T) {
	s, _, _ := newStore(t)
	first := write(t, s, "Acme", 1, UntrustedExternal)
	write(t, s, "Other", 2, UserAsserted)
	got, err := s.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Contested {
		t.Errorf("want contested, got %s", got.Status)
	}
}
