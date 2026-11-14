// Package supervisor runs tasks as ordered steps, checkpointing each one to the
// event log so a restarted run skips steps that already completed.
package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mini-harness/eventlog"
)

// Event types written by the supervisor.
const (
	EventStepStarted   = "step.started"
	EventStepCompleted = "step.completed"
)

// Step is one unit of work. Run receives the step's idempotency key so its side
// effect can be made safe to repeat.
type Step struct {
	Name string
	Run  func(ctx context.Context, key string) error
}

// Task is an ordered list of steps.
type Task struct {
	ID    string
	Steps []Step
}

// Key is the stable idempotency key of step i: task id + index + name.
func (t Task) Key(i int) string {
	return fmt.Sprintf("%s/%d/%s", t.ID, i, t.Steps[i].Name)
}

// Runner executes tasks and records progress in the log.
type Runner struct {
	log     *eventlog.Log
	agentID string

	// Failure policy; tests replace Sleep so backoff costs no real time.
	MaxAttempts int
	BaseBackoff time.Duration
	Sleep       func(time.Duration)
}

// NewRunner returns a Runner that logs under agentID.
func NewRunner(log *eventlog.Log, agentID string) *Runner {
	return &Runner{
		log: log, agentID: agentID,
		MaxAttempts: DefaultMaxAttempts, BaseBackoff: DefaultBaseBackoff, Sleep: time.Sleep,
	}
}

type stepPayload struct {
	Key   string `json:"key"`
	Index int    `json:"index"`
	Name  string `json:"name"`
}

func (r *Runner) record(ctx context.Context, t Task, i int, typ string) error {
	p, err := json.Marshal(stepPayload{Key: t.Key(i), Index: i, Name: t.Steps[i].Name})
	if err != nil {
		return err
	}
	_, err = r.log.Append(ctx, t.ID, r.agentID, typ, string(p))
	return err
}

// completed returns the keys that already have a step.completed checkpoint.
func (r *Runner) completed(ctx context.Context, taskID string) (map[string]bool, error) {
	events, err := r.log.List(ctx, taskID)
	if err != nil {
		return nil, err
	}
	done := map[string]bool{}
	for _, e := range events {
		if e.Type != EventStepCompleted {
			continue
		}
		var p stepPayload
		if err := json.Unmarshal([]byte(e.Payload), &p); err != nil {
			return nil, fmt.Errorf("supervisor: bad checkpoint payload in event %d: %w", e.ID, err)
		}
		done[p.Key] = true
	}
	return done, nil
}

// Run executes the task's steps in order, skipping any with a completed
// checkpoint and recording one after each step that succeeds.
func (r *Runner) Run(ctx context.Context, t Task) error {
	done, err := r.completed(ctx, t.ID)
	if err != nil {
		return err
	}
	for i, s := range t.Steps {
		if s.Run == nil {
			return errors.New("supervisor: step " + s.Name + " has no Run func")
		}
		if done[t.Key(i)] {
			continue
		}
		if err := r.record(ctx, t, i, EventStepStarted); err != nil {
			return err
		}
		if err := r.runStep(ctx, t, i); err != nil {
			return err
		}
		if err := r.record(ctx, t, i, EventStepCompleted); err != nil {
			return err
		}
	}
	return nil
}
