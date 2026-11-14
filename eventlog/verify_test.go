package eventlog

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// filledLog returns a log with 100 events (ids 1..100).
func filledLog(t *testing.T) *Log {
	t.Helper()
	l := openTemp(t, &FakeClock{})
	for i := 1; i <= 100; i++ {
		if _, err := l.Append(context.Background(), "t", "a", "x", fmt.Sprintf(`{"n":%d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

func expectBad(t *testing.T, l *Log, want int64) {
	t.Helper()
	id, err := l.Verify(context.Background())
	if !errors.Is(err, ErrTampered) || id != want {
		t.Fatalf("want first bad id %d with ErrTampered, got id %d err %v", want, id, err)
	}
}

func TestVerifyIntactLog(t *testing.T) {
	l := filledLog(t)
	if id, err := l.Verify(context.Background()); id != 0 || err != nil {
		t.Fatalf("untouched log should verify, got id %d err %v", id, err)
	}
}

func TestVerifyEmptyLog(t *testing.T) {
	l := openTemp(t, nil)
	if id, err := l.Verify(context.Background()); id != 0 || err != nil {
		t.Fatalf("empty log should verify, got id %d err %v", id, err)
	}
}

func TestVerifyPayloadEdit(t *testing.T) {
	l := filledLog(t)
	if _, err := l.db.Exec(`UPDATE events SET payload = '{"n":999}' WHERE id = 42`); err != nil {
		t.Fatal(err)
	}
	expectBad(t, l, 42)
}

func TestVerifyDeletedMiddleRow(t *testing.T) {
	l := filledLog(t)
	if _, err := l.db.Exec(`DELETE FROM events WHERE id = 50`); err != nil {
		t.Fatal(err)
	}
	expectBad(t, l, 51) // first event after the gap
}

func TestVerifyRewrittenHash(t *testing.T) {
	l := filledLog(t)
	if _, err := l.db.Exec(`UPDATE events SET hash = 'deadbeef' WHERE id = 30`); err != nil {
		t.Fatal(err)
	}
	expectBad(t, l, 30)
}

// An attacker who edits a payload and recomputes that event's hash still breaks
// the link held by the next event.
func TestVerifyEditWithRecomputedHash(t *testing.T) {
	l := filledLog(t)
	e, _ := l.Get(context.Background(), 10)
	e.Payload = `{"n":-1}`
	e.Hash = e.ComputeHash()
	if _, err := l.db.Exec(`UPDATE events SET payload = ?, hash = ? WHERE id = 10`, e.Payload, e.Hash); err != nil {
		t.Fatal(err)
	}
	expectBad(t, l, 11)
}
