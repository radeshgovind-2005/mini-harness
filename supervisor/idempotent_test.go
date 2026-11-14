package supervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdempotentSameKeyAppliesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effects.txt")
	for i := 0; i < 3; i++ {
		if err := AppendLineOnce(path, "t1/0/a", "hello"); err != nil {
			t.Fatal(err)
		}
	}
	if err := AppendLineOnce(path, "t1/1/b", "world"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if got, want := string(b), "t1/0/a\thello\nt1/1/b\tworld\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestIdempotentRejectsSeparators(t *testing.T) {
	if err := AppendLineOnce(filepath.Join(t.TempDir(), "f"), "k", "a\nb"); err == nil {
		t.Fatal("expected error for newline in text")
	}
}

// The step applies its effect, then "crashes" before the checkpoint is written.
// On resume the step runs again but the effect must not repeat.
func TestIdempotentCrashBetweenEffectAndCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effects.txt")
	crash := true
	task := Task{ID: "t1", Steps: []Step{{Name: "a", Run: func(ctx context.Context, key string) error {
		if err := AppendLineOnce(path, key, "effect"); err != nil {
			return err
		}
		if crash {
			return errors.New("crash before checkpoint")
		}
		return nil
	}}}}

	r := NewRunner(openLog(t), "agent")
	if err := r.Run(context.Background(), task); err == nil {
		t.Fatal("expected crash")
	}
	crash = false
	if err := r.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if n := strings.Count(string(b), "\n"); n != 1 {
		t.Fatalf("effect applied %d times, want 1: %q", n, b)
	}
}
