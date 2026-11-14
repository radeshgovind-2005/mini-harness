// Package trustgate screens untrusted content and decides whether the agent
// may act on it. It is a defense layer, not a guarantee.
package trustgate

import (
	"regexp"
	"strings"
)

// Content is a piece of text plus whether it is tainted. Incoming content is
// untrusted by default; Tainted is set when a Screen flags it.
type Content struct {
	Text    string
	Source  string // where it came from, e.g. "email:inbox/42"
	Tainted bool
	Reason  string // why it was flagged (empty when clean)
}

// Screen inspects untrusted content before the agent sees it. Callers depend
// only on this interface so the screen can be swapped (e.g. an embedding screen).
type Screen interface {
	Screen(c Content) Content
}

// rule is one named pattern that marks text as instruction-like.
type rule struct {
	name string
	re   *regexp.Regexp
}

// RulesScreen is the first, heuristic Screen: it flags text that reads like
// an instruction aimed at the agent. Easy to bypass; that is a known limit.
type RulesScreen struct {
	rules []rule
}

// NewRulesScreen builds the screen with its fixed rule set.
func NewRulesScreen() *RulesScreen {
	r := func(name, pattern string) rule {
		return rule{name: name, re: regexp.MustCompile(`(?i)` + pattern)}
	}
	return &RulesScreen{rules: []rule{
		r("outbound-instruction", `\b(forward|send|email|share|upload|post)\b.{0,60}\b(to|at)\b.{0,40}(\S+@\S+|https?://)`),
		r("forward-all", `\b(forward|send|share)\s+(all|every|any)\b`),
		r("override-instructions", `\b(ignore|disregard|forget)\b.{0,30}\b(previous|prior|above|earlier|all)\b.{0,20}\b(instructions?|rules?|prompts?)\b`),
		r("role-hijack", `\byou\s+(are\s+now|must\s+now|should\s+now)\b`),
		r("destructive-command", `(rm\s+-rf|\bdelete\s+all\b|\bdrop\s+table\b|curl\s+[^|]*\|\s*(sh|bash)\b)`),
		r("agent-address", `\b(assistant|ai|agent)\b\s*[:,]\s*(please\s+)?(you\s+)?(must|should|do|now)\b`),
	}}
}

// Screen returns c with Tainted/Reason set when any rule matches. A clean
// result keeps whatever taint the content already carried (never un-taints).
func (s *RulesScreen) Screen(c Content) Content {
	var hits []string
	for _, r := range s.rules {
		if r.re.MatchString(c.Text) {
			hits = append(hits, r.name)
		}
	}
	if len(hits) > 0 {
		c.Tainted = true
		c.Reason = "instruction-like content: " + strings.Join(hits, ", ")
	}
	return c
}
