// Package fakemodel is a scripted, deterministic stand-in for an LLM.
// It needs no network and no API key, so tests stay reproducible.
package fakemodel

import (
	"context"
	"errors"
	"sync"
)

// Message is one turn of a conversation sent to a model.
type Message struct {
	Role    string // "user", "assistant", "tool", ...
	Content string
}

// ToolCall is a request from the model to run a named tool.
type ToolCall struct {
	Name string
	Args map[string]string
}

// Response holds either plain Text or a list of ToolCalls.
type Response struct {
	Text      string
	ToolCalls []ToolCall
}

// Model is the minimal interface the harness depends on. A real local model
// adapter can implement it later without changing callers.
type Model interface {
	Complete(ctx context.Context, msgs []Message) (Response, error)
}

// ErrScriptExhausted is returned when Complete is called more times than the
// script has responses.
var ErrScriptExhausted = errors.New("fakemodel: script exhausted")

// Scripted returns its canned responses in order, ignoring the input messages
// (it records them so tests can inspect what the caller sent).
type Scripted struct {
	mu        sync.Mutex
	responses []Response
	next      int
	calls     [][]Message
}

var _ Model = (*Scripted)(nil)

// NewScripted builds a model that replies with responses in order.
func NewScripted(responses ...Response) *Scripted {
	return &Scripted{responses: responses}
}

// Complete returns the next scripted response, or ErrScriptExhausted.
func (s *Scripted) Complete(ctx context.Context, msgs []Message) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, append([]Message(nil), msgs...))
	if s.next >= len(s.responses) {
		return Response{}, ErrScriptExhausted
	}
	r := s.responses[s.next]
	s.next++
	return r, nil
}

// Calls returns the message lists received so far, oldest first.
func (s *Scripted) Calls() [][]Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]Message(nil), s.calls...)
}
