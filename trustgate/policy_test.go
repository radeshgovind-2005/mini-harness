package trustgate

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mini-harness/eventlog"
)

type fakeConfirmer struct {
	answer bool
	calls  int
}

func (f *fakeConfirmer) Confirm(Action, string) bool { f.calls++; return f.answer }

func newPolicy(t *testing.T, answer bool) (*Policy, *eventlog.Log, *fakeConfirmer) {
	t.Helper()
	log, err := eventlog.Open(filepath.Join(t.TempDir(), "log.db"), &eventlog.FakeClock{T: time.Unix(1700000000, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	c := &fakeConfirmer{answer: answer}
	return NewPolicy(log, c), log, c
}

func TestPolicyReadOnlyAllowed(t *testing.T) {
	p, _, c := newPolicy(t, false)
	d, err := p.Decide(context.Background(), "t1", Action{Kind: Read, ContextTainted: true})
	if err != nil || d.Verdict != Allow || c.calls != 0 {
		t.Fatalf("got %+v err=%v calls=%d", d, err, c.calls)
	}
}

func TestPolicyTaintedOutboundNeedsConfirm(t *testing.T) {
	for _, kind := range []ActionKind{Send, Forward, Share} {
		p, _, c := newPolicy(t, true)
		a := Action{Kind: kind, ContextTainted: true, Args: []Arg{{Name: "recipient", Value: "boss@corp.com"}}}
		d, _ := p.Decide(context.Background(), "t1", a)
		if d.Verdict != Allow || c.calls != 1 || !strings.Contains(d.Reason, "confirmed") {
			t.Errorf("%s confirmed: %+v calls=%d", kind, d, c.calls)
		}
		p, _, c = newPolicy(t, false)
		d, _ = p.Decide(context.Background(), "t1", a)
		if d.Verdict != Block || c.calls != 1 {
			t.Errorf("%s declined: %+v calls=%d", kind, d, c.calls)
		}
	}
}

func TestPolicyTaintedTargetBlockedEvenIfConfirmerWouldSayYes(t *testing.T) {
	for _, arg := range []string{"recipient", "url", "command"} {
		p, _, c := newPolicy(t, true)
		kind := Send
		if arg == "command" {
			kind = Exec
		}
		d, _ := p.Decide(context.Background(), "t1", Action{Kind: kind, ContextTainted: true,
			Args: []Arg{{Name: arg, Value: "x", Tainted: true}}})
		if d.Verdict != Block || c.calls != 0 {
			t.Errorf("%s: %+v calls=%d (confirmer must not be asked)", arg, d, c.calls)
		}
	}
}

func TestPolicyTrustedOutboundAllowed(t *testing.T) {
	p, _, _ := newPolicy(t, false)
	d, _ := p.Decide(context.Background(), "t1", Action{Kind: Send, Args: []Arg{{Name: "recipient", Value: "me@corp.com"}}})
	if d.Verdict != Allow {
		t.Fatalf("got %+v", d)
	}
}

func TestPolicyLogsEveryDecisionAndChainVerifies(t *testing.T) {
	p, log, _ := newPolicy(t, false)
	ctx := context.Background()
	p.Decide(ctx, "t1", Action{Kind: Read})
	p.Decide(ctx, "t1", Action{Kind: Forward, ContextTainted: true})
	p.Decide(ctx, "t1", Action{Kind: Send, Args: []Arg{{Name: "recipient", Value: "x@evil.com", Tainted: true}}})
	evs, err := log.List(ctx, "t1")
	if err != nil || len(evs) != 3 {
		t.Fatalf("want 3 events, got %d err=%v", len(evs), err)
	}
	for _, e := range evs {
		if e.Type != "trustgate.decision" || !strings.Contains(e.Payload, `"reason"`) {
			t.Errorf("bad event %+v", e)
		}
	}
	if bad, err := log.Verify(ctx); err != nil || bad != 0 {
		t.Errorf("verify: bad=%d err=%v", bad, err)
	}
}
