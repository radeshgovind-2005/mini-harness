package eventlog

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadGetAndList(t *testing.T) {
	clock := &FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := openTemp(t, clock)
	ctx := context.Background()

	var appended []Event
	for _, task := range []string{"a", "b", "a"} {
		e, err := l.Append(ctx, task, "ag", "x", `{}`)
		if err != nil {
			t.Fatal(err)
		}
		appended = append(appended, e)
		clock.Advance(time.Second)
	}

	got, err := l.Get(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != appended[1] {
		t.Fatalf("Get mismatch: %+v vs %+v", got, appended[1])
	}
	if _, err := l.Get(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	list, err := l.List(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 3 {
		t.Fatalf("List(a) should be ids 1,3 in order, got %+v", list)
	}
	all, err := l.List(ctx, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("List(all): %v, %d events", err, len(all))
	}
	if none, _ := l.List(ctx, "zzz"); len(none) != 0 {
		t.Fatal("unknown task should list nothing")
	}
}
