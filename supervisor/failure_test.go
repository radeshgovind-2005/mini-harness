package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// failingTask has one step that returns errs[n] on attempt n and nil after.
func failingTask(attempts *int, errs ...error) Task {
	return Task{ID: "f1", Steps: []Step{{Name: "s", Run: func(ctx context.Context, key string) error {
		n := *attempts
		*attempts++
		if n < len(errs) {
			return errs[n]
		}
		return nil
	}}}}
}

func testRunner(t *testing.T) (*Runner, *[]time.Duration) {
	r := NewRunner(openLog(t), "agent")
	var sleeps []time.Duration
	r.Sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	return r, &sleeps
}

func eventCount(t *testing.T, r *Runner, task, typ string) int {
	events, err := r.log.List(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

func TestFailureTransientClearsThenSucceeds(t *testing.T) {
	r, sleeps := testRunner(t)
	attempts := 0
	boom := Transient(errors.New("timeout"))
	if err := r.Run(context.Background(), failingTask(&attempts, boom, boom, boom)); err != nil {
		t.Fatal(err)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	if len(*sleeps) != len(want) {
		t.Fatalf("sleeps = %v, want %v", *sleeps, want)
	}
	for i := range want {
		if (*sleeps)[i] != want[i] {
			t.Fatalf("sleeps = %v, want %v", *sleeps, want)
		}
	}
	if n := eventCount(t, r, "f1", EventStepRetry); n != 3 {
		t.Fatalf("retry events = %d, want 3", n)
	}
	if n := eventCount(t, r, "f1", EventStepCompleted); n != 1 {
		t.Fatalf("completed events = %d, want 1", n)
	}
}

func TestFailureTransientCapEscalates(t *testing.T) {
	r, sleeps := testRunner(t)
	attempts := 0
	boom := Transient(errors.New("down"))
	err := r.Run(context.Background(), failingTask(&attempts, boom, boom, boom, boom, boom, boom, boom))
	var esc *EscalatedError
	if !errors.As(err, &esc) {
		t.Fatalf("err = %v, want EscalatedError", err)
	}
	if attempts != DefaultMaxAttempts || esc.Attempts != DefaultMaxAttempts {
		t.Fatalf("attempts = %d / %d, want %d", attempts, esc.Attempts, DefaultMaxAttempts)
	}
	if len(*sleeps) != DefaultMaxAttempts-1 {
		t.Fatalf("sleeps = %v", *sleeps)
	}
	if n := eventCount(t, r, "f1", EventStepEscalated); n != 1 {
		t.Fatalf("escalated events = %d, want 1", n)
	}
	if n := eventCount(t, r, "f1", EventStepCompleted); n != 0 {
		t.Fatal("failed step must not be checkpointed")
	}
}

func TestFailurePermanentOneAttempt(t *testing.T) {
	r, sleeps := testRunner(t)
	attempts := 0
	err := r.Run(context.Background(), failingTask(&attempts, errors.New("forbidden")))
	var esc *EscalatedError
	if !errors.As(err, &esc) {
		t.Fatalf("err = %v, want EscalatedError", err)
	}
	if attempts != 1 || len(*sleeps) != 0 {
		t.Fatalf("attempts = %d, sleeps = %v; want 1 and none", attempts, *sleeps)
	}
	if n := eventCount(t, r, "f1", EventStepEscalated); n != 1 {
		t.Fatalf("escalated events = %d, want 1", n)
	}
}

func TestFailureFixTheCallReturnsTeachingError(t *testing.T) {
	r, sleeps := testRunner(t)
	attempts := 0
	err := r.Run(context.Background(), failingTask(&attempts, FixTheCall(errors.New("date must be YYYY-MM-DD"))))
	var te *TeachingError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want TeachingError", err)
	}
	if te.Err.Error() != "date must be YYYY-MM-DD" {
		t.Fatalf("teaching message = %q", te.Err)
	}
	if attempts != 1 || len(*sleeps) != 0 {
		t.Fatalf("attempts = %d, sleeps = %v; want 1 and none", attempts, *sleeps)
	}
	if n := eventCount(t, r, "f1", EventStepFixCall); n != 1 {
		t.Fatalf("fix_the_call events = %d, want 1", n)
	}
	if n := eventCount(t, r, "f1", EventStepEscalated); n != 0 {
		t.Fatal("fix-the-call must not be logged as escalation")
	}
}
