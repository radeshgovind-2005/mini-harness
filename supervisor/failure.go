package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Event types for failure handling.
const (
	EventStepRetry     = "step.retry"
	EventStepFixCall   = "step.fix_the_call"
	EventStepEscalated = "step.escalated"
)

// Defaults from specs/decisions.md.
const (
	DefaultMaxAttempts = 5
	DefaultBaseBackoff = 100 * time.Millisecond
)

// Class says how the supervisor reacts to a step error.
type Class int

const (
	// ClassEscalate is the default: permanent or policy failure, stop, no retry.
	ClassEscalate Class = iota
	// ClassRetry is transient: back off and try again, up to the cap.
	ClassRetry
	// ClassFixTheCall is model-correctable: hand a teaching error back to the caller.
	ClassFixTheCall
)

func (c Class) String() string {
	switch c {
	case ClassRetry:
		return "retry"
	case ClassFixTheCall:
		return "fix-the-call"
	default:
		return "escalate"
	}
}

// classified marks an error with a Class. Unmarked errors escalate.
type classified struct {
	class Class
	err   error
}

func (e *classified) Error() string { return e.err.Error() }
func (e *classified) Unwrap() error { return e.err }

// Transient marks err as worth retrying with backoff.
func Transient(err error) error { return &classified{ClassRetry, err} }

// FixTheCall marks err as a mistake the caller (the model) can correct. The
// message should say what was wrong and how to fix it.
func FixTheCall(err error) error { return &classified{ClassFixTheCall, err} }

// Classify returns the class of err; anything unmarked is ClassEscalate.
func Classify(err error) Class {
	var c *classified
	if errors.As(err, &c) {
		return c.class
	}
	return ClassEscalate
}

// TeachingError is what Run returns for a fix-the-call failure. It is not a
// crash: the task can be re-run once the call is corrected.
type TeachingError struct {
	Step string
	Err  error
}

func (e *TeachingError) Error() string {
	return fmt.Sprintf("supervisor: step %q needs a corrected call: %v", e.Step, e.Err)
}
func (e *TeachingError) Unwrap() error { return e.Err }

// EscalatedError is what Run returns when a step is permanent or retries ran out.
type EscalatedError struct {
	Step     string
	Attempts int
	Err      error
}

func (e *EscalatedError) Error() string {
	return fmt.Sprintf("supervisor: step %q escalated after %d attempt(s): %v", e.Step, e.Attempts, e.Err)
}
func (e *EscalatedError) Unwrap() error { return e.Err }

type failurePayload struct {
	Key     string `json:"key"`
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Attempt int    `json:"attempt"`
	Error   string `json:"error"`
	// BackoffMS is only set on retry events.
	BackoffMS int64 `json:"backoff_ms,omitempty"`
}

func (r *Runner) recordFailure(ctx context.Context, t Task, i int, typ string, attempt int, err error, backoff time.Duration) error {
	p, merr := json.Marshal(failurePayload{
		Key: t.Key(i), Index: i, Name: t.Steps[i].Name,
		Attempt: attempt, Error: err.Error(), BackoffMS: backoff.Milliseconds(),
	})
	if merr != nil {
		return merr
	}
	_, aerr := r.log.Append(ctx, t.ID, r.agentID, typ, string(p))
	return aerr
}

// runStep runs step i, applying the failure policy to its errors.
func (r *Runner) runStep(ctx context.Context, t Task, i int) error {
	s := t.Steps[i]
	backoff := r.BaseBackoff
	for attempt := 1; ; attempt++ {
		err := s.Run(ctx, t.Key(i))
		if err == nil {
			return nil
		}
		switch Classify(err) {
		case ClassFixTheCall:
			if lerr := r.recordFailure(ctx, t, i, EventStepFixCall, attempt, err, 0); lerr != nil {
				return lerr
			}
			return &TeachingError{Step: s.Name, Err: err}
		case ClassRetry:
			if attempt < r.MaxAttempts {
				if lerr := r.recordFailure(ctx, t, i, EventStepRetry, attempt, err, backoff); lerr != nil {
					return lerr
				}
				r.Sleep(backoff)
				backoff *= 2
				continue
			}
		}
		// Permanent, or transient with the cap reached.
		if lerr := r.recordFailure(ctx, t, i, EventStepEscalated, attempt, err, 0); lerr != nil {
			return lerr
		}
		return &EscalatedError{Step: s.Name, Attempts: attempt, Err: err}
	}
}
