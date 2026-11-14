package trustgate

import (
	"context"
	"encoding/json"
	"fmt"

	"mini-harness/eventlog"
)

// Verdict is the policy's answer for one requested action.
type Verdict string

const (
	Allow   Verdict = "allow"
	Confirm Verdict = "confirm"
	Block   Verdict = "block"
)

// ActionKind groups actions by how risky they are.
type ActionKind string

const (
	Read    ActionKind = "read"    // read-only, e.g. summarize
	Send    ActionKind = "send"    // outbound
	Forward ActionKind = "forward" // outbound
	Share   ActionKind = "share"   // outbound
	Delete  ActionKind = "delete"  // destructive
	Exec    ActionKind = "exec"    // destructive
)

func (k ActionKind) outbound() bool    { return k == Send || k == Forward || k == Share }
func (k ActionKind) destructive() bool { return k == Delete || k == Exec }

// Arg is one argument of an action and whether its value came from tainted content.
type Arg struct {
	Name    string // "recipient", "url", "command", ...
	Value   string
	Tainted bool
}

// Action is what the agent asks to do. ContextTainted means the agent has
// seen tainted content while deciding on this action.
type Action struct {
	Kind           ActionKind
	Args           []Arg
	ContextTainted bool
}

// targetArgs must come from a trusted source for outbound/destructive actions.
var targetArgs = map[string]bool{"recipient": true, "url": true, "command": true}

// Confirmer asks a human to approve an action. Tests use a fake.
type Confirmer interface {
	Confirm(a Action, reason string) bool
}

// Decision is the final verdict plus why.
type Decision struct {
	Verdict Verdict
	Reason  string
}

// Policy decides allow / confirm / block and logs every decision.
type Policy struct {
	log       *eventlog.Log
	confirmer Confirmer
}

func NewPolicy(log *eventlog.Log, confirmer Confirmer) *Policy {
	return &Policy{log: log, confirmer: confirmer}
}

// Evaluate is the pure rule check: it never asks the human or logs.
func Evaluate(a Action) Decision {
	risky := a.Kind.outbound() || a.Kind.destructive()
	if !risky {
		return Decision{Allow, "read-only action"}
	}
	// Tainted content must never choose where an action goes or what it runs,
	// so a confirm cannot rescue it.
	for _, arg := range a.Args {
		if arg.Tainted && targetArgs[arg.Name] {
			return Decision{Block, fmt.Sprintf("%s of a %s action came from tainted content", arg.Name, a.Kind)}
		}
	}
	if a.ContextTainted {
		return Decision{Confirm, fmt.Sprintf("%s action requested after reading tainted content", a.Kind)}
	}
	return Decision{Allow, "trusted action with trusted arguments"}
}

// Decide evaluates the action, asks the confirmer when needed, and logs the
// final decision (with its reason) to the event log.
func (p *Policy) Decide(ctx context.Context, taskID string, a Action) (Decision, error) {
	d := Evaluate(a)
	if d.Verdict == Confirm {
		if p.confirmer != nil && p.confirmer.Confirm(a, d.Reason) {
			d = Decision{Allow, d.Reason + "; human confirmed"}
		} else {
			d = Decision{Block, d.Reason + "; human declined"}
		}
	}
	payload, err := json.Marshal(map[string]any{
		"kind": a.Kind, "args": a.Args, "verdict": d.Verdict, "reason": d.Reason,
	})
	if err != nil {
		return d, err
	}
	if _, err := p.log.Append(ctx, taskID, "trustgate", "trustgate.decision", string(payload)); err != nil {
		return d, err
	}
	return d, nil
}
