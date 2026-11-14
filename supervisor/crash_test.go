package supervisor

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const crashRuns = 50

// killWorkerAt starts the worker paused after step k's effect (before its
// checkpoint), waits until it says so, then SIGKILLs the real process.
func killWorkerAt(t *testing.T, bin, db, effects string, k int) {
	t.Helper()
	cmd := exec.Command(bin, "-db", db, "-effects", effects, "-pause-at", fmt.Sprint(k))
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("paused %d", k)
	sc := bufio.NewScanner(out)
	if !sc.Scan() || sc.Text() != want {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatalf("worker did not report %q (got %q)", want, sc.Text())
	}
	if err := cmd.Process.Kill(); err != nil { // SIGKILL, not a graceful stop
		t.Fatal(err)
	}
	cmd.Wait() // returns "signal: killed"; expected
}

func TestCrashRecoveryExactlyOnce(t *testing.T) {
	bin := buildWorker(t)
	rng := rand.New(rand.NewSource(1)) // seeded: same kill steps every run
	passes := 0

	for run := 0; run < crashRuns; run++ {
		dir := t.TempDir()
		db, effects := filepath.Join(dir, "log.db"), filepath.Join(dir, "effects.txt")
		k := rng.Intn(10)

		killWorkerAt(t, bin, db, effects, k)
		// Restart with no pause hook until the task finishes.
		if out, err := exec.Command(bin, "-db", db, "-effects", effects).CombinedOutput(); err != nil {
			t.Errorf("run %d (kill at step %d): restart failed: %v\n%s", run, k, err, out)
			continue
		}

		b, err := os.ReadFile(effects)
		if err != nil {
			t.Errorf("run %d: %v", run, err)
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		ok := len(lines) == 10
		for i := 0; ok && i < 10; i++ {
			// Lines are "key<TAB>text" in step order, each step exactly once.
			if !strings.HasSuffix(lines[i], fmt.Sprintf("\tline %d", i)) {
				ok = false
			}
		}
		if !ok {
			t.Errorf("run %d (kill at step %d): effects not exactly once in order:\n%s", run, k, b)
			continue
		}
		passes++
	}
	t.Logf("crash test: runs=%d passes=%d", crashRuns, passes)
	if passes != crashRuns {
		t.Fatalf("passes=%d, want %d", passes, crashRuns)
	}
}
