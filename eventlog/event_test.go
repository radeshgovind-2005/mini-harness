package eventlog

import (
	"testing"
	"time"
)

func sampleEvent() Event {
	return Event{
		ID: 1, TaskID: "t1", AgentID: "a1", Type: "tool.call",
		Payload:   `{"x":1}`,
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		PrevHash:  GenesisHash,
	}
}

func TestHashDeterministic(t *testing.T) {
	a, b := sampleEvent(), sampleEvent()
	if a.ComputeHash() != b.ComputeHash() {
		t.Fatal("same fields must hash the same")
	}
	if len(a.ComputeHash()) != 64 {
		t.Fatalf("want 64 hex chars, got %d", len(a.ComputeHash()))
	}
}

func TestHashCoversEveryField(t *testing.T) {
	base := sampleEvent().ComputeHash()
	mutations := map[string]func(*Event){
		"id":        func(e *Event) { e.ID = 2 },
		"task":      func(e *Event) { e.TaskID = "t2" },
		"agent":     func(e *Event) { e.AgentID = "a2" },
		"type":      func(e *Event) { e.Type = "other" },
		"payload":   func(e *Event) { e.Payload = `{"x":2}` },
		"timestamp": func(e *Event) { e.Timestamp = e.Timestamp.Add(time.Nanosecond) },
		"prev":      func(e *Event) { e.PrevHash = "abc" },
	}
	for name, mutate := range mutations {
		e := sampleEvent()
		mutate(&e)
		if e.ComputeHash() == base {
			t.Errorf("changing %s did not change the hash", name)
		}
	}
}

func TestHashFieldBoundaries(t *testing.T) {
	a, b := sampleEvent(), sampleEvent()
	a.TaskID, a.AgentID = "a", "bc"
	b.TaskID, b.AgentID = "ab", "c"
	if a.ComputeHash() == b.ComputeHash() {
		t.Fatal("shifting a field boundary must change the hash")
	}
}

func TestHashGenesisIsZero(t *testing.T) {
	if len(GenesisHash) != 64 {
		t.Fatalf("genesis hash length %d", len(GenesisHash))
	}
	for _, c := range GenesisHash {
		if c != '0' {
			t.Fatal("genesis hash must be all zeros")
		}
	}
}

func TestFakeClock(t *testing.T) {
	c := &FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	c.Advance(time.Minute)
	if got := c.Now(); !got.Equal(time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)) {
		t.Fatalf("got %v", got)
	}
}
