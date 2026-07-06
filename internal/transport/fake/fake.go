/* Package fake is an in-memory Transport for tests. */
package fake

import (
	"hop/internal/bundle"
	"hop/internal/transport"
)

/* Fake stores bundles in memory, keyed by project ID. */
type Fake struct {
	store map[string]*bundle.Bundle
}

/* New returns an empty in-memory transport. */
func New() *Fake { return &Fake{store: map[string]*bundle.Bundle{}} }

/* Send stores a copy-by-reference of the bundle. */
func (f *Fake) Send(b *bundle.Bundle) error {
	f.store[b.Meta.ProjectID] = b
	return nil
}

/* Receive returns the stored bundle or transport.ErrNoBundle. */
func (f *Fake) Receive(projectID string) (*bundle.Bundle, error) {
	b, ok := f.store[projectID]
	if !ok {
		return nil, transport.ErrNoBundle
	}
	return b, nil
}

var _ transport.Transport = (*Fake)(nil)
