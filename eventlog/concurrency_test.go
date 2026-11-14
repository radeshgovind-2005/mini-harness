package eventlog

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// Many writers append at once; the chain must stay linear and verify.
func TestConcurrentAppendNoFork(t *testing.T) {
	l := openTemp(t, &FakeClock{})
	ctx := context.Background()
	const writers, each = 20, 10

	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				payload := fmt.Sprintf(`{"w":%d,"i":%d}`, w, i)
				if _, err := l.Append(ctx, "t", fmt.Sprintf("a%d", w), "x", payload); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	events, err := l.List(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != writers*each {
		t.Fatalf("want %d events, got %d", writers*each, len(events))
	}
	for i, e := range events {
		if e.ID != int64(i+1) {
			t.Fatalf("ids must be contiguous, got %d at position %d", e.ID, i)
		}
	}
	if id, err := l.Verify(ctx); err != nil {
		t.Fatalf("chain should verify, bad id %d: %v", id, err)
	}
}

// The log type must offer no way to rewrite history.
func TestNoUpdateOrDeleteMethods(t *testing.T) {
	typ := reflect.TypeOf(&Log{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete") {
			t.Errorf("Log must not export %s", name)
		}
	}
}
