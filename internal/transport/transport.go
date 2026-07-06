/*
	Package transport moves a bundle between machines.

Implementations differ only in the medium; the core never knows which one is in use.
*/
package transport

import (
	"errors"

	"hop/internal/bundle"
)

/* ErrNoBundle means the transport holds no bundle for the given project yet. */
var ErrNoBundle = errors.New("hop: no bundle found for project")

/* ErrIncompleteBundle means the transport found a bundle whose manifest does not match its files on disk. */
var ErrIncompleteBundle = errors.New("hop: bundle is incomplete or still syncing")

/* Transport sends and receives a project's bundle. */
type Transport interface {
	Send(b *bundle.Bundle) error
	Receive(projectID string) (*bundle.Bundle, error)
}
