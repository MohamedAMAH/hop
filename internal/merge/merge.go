/*
Package merge decides how to reconcile a local and incoming transcript for
one session, relying on transcripts being append-only.
*/
package merge

import "bytes"

/* Decision is the outcome of comparing one session's local and incoming bytes. */
type Decision int

const (
	New            Decision = iota // Incoming exists, local does not.
	KeepLocalOnly                  // Local exists, incoming does not.
	NoOp                           // Identical.
	Update                         // Local is a prefix of incoming (normal hand-off).
	KeepLocalNewer                 // Incoming is a prefix of local (stale bundle).
	Diverged                       // Neither is a prefix of the other.
)

/* Decide compares local and incoming (either may be nil/empty-absent). */
func Decide(local, incoming []byte) Decision {
	switch {
	case local == nil && incoming == nil:
		return NoOp
	case local == nil:
		return New
	case incoming == nil:
		return KeepLocalOnly
	}
	if bytes.Equal(local, incoming) {
		return NoOp
	}
	if bytes.HasPrefix(incoming, local) {
		return Update
	}
	if bytes.HasPrefix(local, incoming) {
		return KeepLocalNewer
	}
	return Diverged
}
