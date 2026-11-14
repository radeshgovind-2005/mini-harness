package supervisor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mini-harness/eventlog"
)

func openLog(t *testing.T) *eventlog.Log {
	t.Helper()
	l, err := eventlog.Open(filepath.Join(t.TempDir(), "log.db"), &eventlog.FakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

// countingTask has three steps that count their runs by key; step failAt errors.
func countingTask(counts map[string]int, failAt int) Task {
	t := Task{ID: "t1"}
	for i, name := range []string{"a", "b", "c"} {
		t.Steps = append(t.Steps, Step{Name: name, Run: func(ctx context.Context, key string) error {
			if i == failAt {
				return errors.New("boom")
			}
			counts[key]++
			return nil
		}})
	}
	return t
}

func TestCheckpointKey(t *testing.T) {
	task := countingTask(map[string]int{}, -1)
	if got := task.Key(1); got != "t1/1/b" {
		t.Fatalf("key = %q", got)
	}
}

func TestCheckpointSkipsCompletedSteps(t *testing.T) {
	r := NewRunner(openLog(t), "agent")
	counts := map[string]int{}

	// First run fails at step 2: steps 0 and 1 are checkpointed.
	if err := r.Run(context.Background(), countingTask(counts, 2)); err == nil {
		t.Fatal("expected failure")
	}
	// Restart with the failure gone: only step 2 runs.
	if err := r.Run(context.Background(), countingTask(counts, -1)); err != nil {
		t.Fatal(err)
	}
	// A third run does nothing.
	if err := r.Run(context.Background(), countingTask(counts, -1)); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"t1/0/a", "t1/1/b", "t1/2/c"} {
		if counts[k] != 1 {
			t.Errorf("%s ran %d times, want 1", k, counts[k])
		}
	}
}

func TestCheckpointLoggedAfterEachStep(t *testing.T) {
	log := openLog(t)
	if err := NewRunner(log, "agent").Run(context.Background(), countingTask(map[string]int{}, -1)); err != nil {
		t.Fatal(err)
	}
	events, err := log.List(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	var started, completed int
	for _, e := range events {
		switch e.Type {
		case EventStepStarted:
			started++
		case EventStepCompleted:
			completed++
		}
	}
	if started != 3 || completed != 3 {
		t.Fatalf("started=%d completed=%d, want 3 each", started, completed)
	}
}
