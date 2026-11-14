// Package eventlog is an append-only, hash-chained event log backed by SQLite.
package eventlog

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// GenesisHash is the "previous hash" of the first event in a log.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Clock supplies timestamps so tests can be deterministic.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the real wall clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// FakeClock returns a fixed time that tests move forward with Advance.
type FakeClock struct {
	T time.Time
}

func (c *FakeClock) Now() time.Time { return c.T }

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) { c.T = c.T.Add(d) }

// Event is one entry in the log. Hash covers every other field, including PrevHash.
type Event struct {
	ID        int64
	TaskID    string
	AgentID   string
	Type      string
	Payload   string // JSON text
	Timestamp time.Time
	PrevHash  string
	Hash      string
}

// ComputeHash returns the hex SHA-256 of all fields except Hash.
// Each string is length-prefixed so field boundaries can't be shifted
// (e.g. task "a"+agent "bc" vs task "ab"+agent "c" hash differently).
func (e Event) ComputeHash() string {
	h := sha256.New()
	writeInt := func(n int64) {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(n))
		h.Write(b[:])
	}
	writeStr := func(s string) {
		writeInt(int64(len(s)))
		h.Write([]byte(s))
	}
	writeInt(e.ID)
	writeStr(e.TaskID)
	writeStr(e.AgentID)
	writeStr(e.Type)
	writeStr(e.Payload)
	writeInt(e.Timestamp.UnixNano())
	writeStr(e.PrevHash)
	return hex.EncodeToString(h.Sum(nil))
}
