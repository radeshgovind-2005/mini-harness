// Package memory stores what the agent knows as claims (subject, predicate, object)
// with provenance and time, instead of free-form prose.
package memory

import "time"

// SourceKind says where a claim came from. It decides the starting confidence and status.
type SourceKind string

const (
	UserAsserted      SourceKind = "user_asserted"
	DocumentDerived   SourceKind = "document_derived"
	UntrustedExternal SourceKind = "untrusted_external"
	ModelInferred     SourceKind = "model_inferred"
)

// Status is where a claim is in its life.
type Status string

const (
	Active     Status = "active"
	Superseded Status = "superseded"
	Contested  Status = "contested" // not yet confirmed; never returned as a fact
)

// Starting confidence per source kind. Promotion raises untrusted claims to PromotedConfidence.
const (
	ConfUserAsserted      = 0.9
	ConfDocumentDerived   = 0.7
	ConfModelInferred     = 0.4
	ConfUntrustedExternal = 0.3
	PromotedConfidence    = 0.6
)

// Claim is one stored fact. A zero ValidTo means the claim is still open-ended.
type Claim struct {
	ID         int64
	Subject    string
	Predicate  string
	Object     string
	SourceID   int64 // eventlog event id this claim came from
	SourceKind SourceKind
	Confidence float64
	ValidFrom  time.Time
	ValidTo    time.Time
	RecordedAt time.Time
	Status     Status
}

// defaultConfidence maps a source kind to its starting confidence.
func defaultConfidence(k SourceKind) (float64, bool) {
	switch k {
	case UserAsserted:
		return ConfUserAsserted, true
	case DocumentDerived:
		return ConfDocumentDerived, true
	case ModelInferred:
		return ConfModelInferred, true
	case UntrustedExternal:
		return ConfUntrustedExternal, true
	}
	return 0, false
}
