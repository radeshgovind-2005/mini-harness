package trustgate

import (
	"context"
	"fmt"

	"mini-harness/memory"
)

// Fact is a subject/predicate/object triple a model extracted from content.
type Fact struct {
	Subject, Predicate, Object string
}

// RememberFacts stores facts extracted from untrusted content as
// untrusted_external claims. The store gives them low confidence and the
// contested status, so they never read back as confirmed facts until a second
// independent source corroborates them. sourceID is the event log id of the
// event that recorded the content.
func RememberFacts(ctx context.Context, store *memory.Store, sourceID int64, facts []Fact) ([]memory.Claim, error) {
	var out []memory.Claim
	for _, f := range facts {
		c, err := store.Write(ctx, memory.Claim{
			Subject: f.Subject, Predicate: f.Predicate, Object: f.Object,
			SourceID: sourceID, SourceKind: memory.UntrustedExternal,
		})
		if err != nil {
			return out, fmt.Errorf("trustgate: remember %q: %w", f.Subject, err)
		}
		out = append(out, c)
	}
	return out, nil
}
