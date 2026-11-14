package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildWorker compiles cmd/worker into a temp dir and returns its path.
func buildWorker(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "worker")
	if out, err := exec.Command("go", "build", "-o", bin, "mini-harness/cmd/worker").CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v\n%s", err, out)
	}
	return bin
}

func TestWorkerRunsTenStepsAndRerunIsNoop(t *testing.T) {
	bin := buildWorker(t)
	dir := t.TempDir()
	db, effects := filepath.Join(dir, "log.db"), filepath.Join(dir, "effects.txt")

	// The second run resumes from the checkpoints and must add nothing.
	for run := 0; run < 2; run++ {
		if out, err := exec.Command(bin, "-db", db, "-effects", effects).CombinedOutput(); err != nil {
			t.Fatalf("run %d: %v\n%s", run, err, out)
		}
		b, _ := os.ReadFile(effects)
		if n := len(strings.Split(strings.TrimSpace(string(b)), "\n")); n != 10 {
			t.Fatalf("run %d: %d lines, want 10", run, n)
		}
	}
}

func TestWorkerRequiresFlags(t *testing.T) {
	if err := exec.Command(buildWorker(t)).Run(); err == nil {
		t.Fatal("expected failure without flags")
	}
}
