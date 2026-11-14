package eventlog

import (
	"context"
	"errors"
	"fmt"
)

// ErrTampered is wrapped by the error Verify returns when the chain is broken.
var ErrTampered = errors.New("eventlog: chain broken")

// Verify walks the log from the start. It returns (0, nil) for an intact chain,
// or the id of the first bad event together with an error wrapping ErrTampered.
// An event is bad if its hash doesn't match its fields, it doesn't link to the
// previous event's hash, or its id leaves a gap (a deleted row).
// Note: deleting the newest events leaves a valid shorter chain and can't be detected here.
func (l *Log) Verify(ctx context.Context) (int64, error) {
	events, err := l.List(ctx, "")
	if err != nil {
		return 0, err
	}
	prevID, prevHash := int64(0), GenesisHash
	for _, e := range events {
		switch {
		case e.ID != prevID+1:
			return e.ID, fmt.Errorf("%w: event %d follows %d (missing row)", ErrTampered, e.ID, prevID)
		case e.PrevHash != prevHash:
			return e.ID, fmt.Errorf("%w: event %d prev hash does not match event %d", ErrTampered, e.ID, prevID)
		case e.Hash != e.ComputeHash():
			return e.ID, fmt.Errorf("%w: event %d hash does not match its contents", ErrTampered, e.ID)
		}
		prevID, prevHash = e.ID, e.Hash
	}
	return 0, nil
}
