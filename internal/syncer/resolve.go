package syncer

/*
Resolution is a decision about a single diverged session during a pull.
*/
type Resolution int

const (
	Abort        Resolution = iota // stop the pull and change nothing further.
	KeepLocal                      // keep this machine's version and skip the incoming one.
	KeepIncoming                   // back up the local session and overwrite it with the incoming one.
)

/*
Resolver decides how to handle a session whose local and incoming transcripts
have genuinely forked.
*/
type Resolver interface {
	Resolve(id string, local, incoming []byte) (Resolution, error)
}

/* AbortResolver stops the pull on any divergence. */
type AbortResolver struct{}

/* Resolve always returns Abort. */
func (AbortResolver) Resolve(string, []byte, []byte) (Resolution, error) { return Abort, nil }

/* ForceResolver overwrites local with incoming on any divergence and forces past a stale-pull. */
type ForceResolver struct{}

/* Resolve always returns KeepIncoming. */
func (ForceResolver) Resolve(string, []byte, []byte) (Resolution, error) { return KeepIncoming, nil }

/* forces reports that this resolver bypasses the stale-pull guard. */
func (ForceResolver) forces() bool { return true }

/* forcer is implemented by resolvers that bypass the stale-pull guard. */
type forcer interface{ forces() bool }

/* forcesStalePull reports whether a resolver opts to bypass the stale-pull guard. */
func forcesStalePull(r Resolver) bool {
	f, ok := r.(forcer)
	return ok && f.forces()
}
