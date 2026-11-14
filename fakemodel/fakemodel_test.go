package fakemodel

import (
	"context"
	"errors"
	"testing"
)

func TestScriptedReturnsInOrder(t *testing.T) {
	m := NewScripted(
		Response{ToolCalls: []ToolCall{{Name: "read_email", Args: map[string]string{"id": "1"}}}},
		Response{Text: "done"},
	)
	ctx := context.Background()

	r1, err := m.Complete(ctx, []Message{{Role: "user", Content: "hi"}})
	if err != nil || len(r1.ToolCalls) != 1 || r1.ToolCalls[0].Name != "read_email" {
		t.Fatalf("first response = %+v, %v", r1, err)
	}
	r2, err := m.Complete(ctx, nil)
	if err != nil || r2.Text != "done" {
		t.Fatalf("second response = %+v, %v", r2, err)
	}
	if _, err := m.Complete(ctx, nil); !errors.Is(err, ErrScriptExhausted) {
		t.Fatalf("want ErrScriptExhausted, got %v", err)
	}
}

func TestScriptedIsDeterministicAndRecordsCalls(t *testing.T) {
	run := func() []string {
		m := NewScripted(Response{Text: "a"}, Response{Text: "b"})
		var out []string
		for i := 0; i < 2; i++ {
			r, _ := m.Complete(context.Background(), []Message{{Role: "user", Content: "x"}})
			out = append(out, r.Text)
		}
		if len(m.Calls()) != 2 {
			t.Fatalf("calls = %d, want 2", len(m.Calls()))
		}
		return out
	}
	a, b := run(), run()
	if a[0] != b[0] || a[1] != b[1] {
		t.Fatalf("runs differ: %v vs %v", a, b)
	}
}

func TestScriptedHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewScripted(Response{Text: "a"})
	if _, err := m.Complete(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
