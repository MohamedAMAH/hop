/*
Package syncer orchestrates push and pull: capture, neutralize, baton, and
state updates on top of the agent and transport interfaces.
*/
package syncer

import (
	"errors"
	"fmt"
	"path/filepath"

	"hop/internal/agent"
	"hop/internal/bundle"
	"hop/internal/config"
	"hop/internal/merge"
	"hop/internal/osinfo"
	"hop/internal/rewrite"
	"hop/internal/state"
	"hop/internal/transport"
)

/* ErrDiverged means both machines have advanced independently. */
var ErrDiverged = errors.New("hop: sessions diverged between machines")

/* Deps carries everything an operation needs. */
type Deps struct {
	Cfg       config.Config
	Agent     agent.Agent
	Transport transport.Transport
	Home      string
	StateDir  string
	OS        osinfo.OS
	/* Notify, when set, receives informational notices such as the both-advanced heads-up. */
	Notify func(string)
}

/* Report summarizes an operation for display. */
type Report struct {
	ProjectID string
	Sessions  int
	Sequence  int
}

/* statePath returns the path to projectID's local state file. */
func (d Deps) statePath(projectID string) string {
	return filepath.Join(d.StateDir, projectID+".json")
}

/* localRoot returns the project's absolute path on this machine. */
func (d Deps) localRoot(projectID string) (string, error) {
	root, ok := d.Cfg.PathFor(projectID, d.Cfg.Machine)
	if !ok {
		return "", fmt.Errorf("hop: project %q is not configured on machine %q; run `hop init`", projectID, d.Cfg.Machine)
	}
	return root, nil
}

/*
Push captures local sessions, neutralizes them, releases and bumps the
baton, sends the bundle, and refreshes local state. now is an RFC3339 stamp.
*/
func Push(d Deps, projectID, now string) (Report, error) {
	root, err := d.localRoot(projectID)
	if err != nil {
		return Report{}, err
	}
	sessions, err := d.Agent.ListSessions(d.Home, root)
	if err != nil {
		return Report{}, err
	}

	st, err := state.Load(d.statePath(projectID))
	if err != nil {
		return Report{}, err
	}

	// Divergence: if the remote advanced past what we last synced AND we also
	// have local changes, both moved. A remote holding no sessions has no work a
	// push could discard, so it never counts as divergence.
	remoteSeq := 0
	remoteHasSessions := false
	if rb, rerr := d.Transport.Receive(projectID); rerr == nil {
		remoteSeq = rb.Meta.Baton.Sequence
		remoteHasSessions = len(rb.Sessions) > 0
	} else if !errors.Is(rerr, transport.ErrNoBundle) {
		return Report{}, rerr
	}
	if remoteHasSessions && remoteSeq != st.LastSyncedSequence && st.Dirty(sessions) {
		return Report{}, ErrDiverged
	}

	token := bundle.SelectToken(sessions)
	neutral := make([]agent.Session, 0, len(sessions))
	for _, s := range sessions {
		data, err := rewrite.Neutralize(s.Data, root, d.OS, token)
		if err != nil {
			return Report{}, err
		}
		neutral = append(neutral, agent.Session{ID: s.ID, Data: data})
	}

	seq := st.LastSyncedSequence
	if remoteSeq > seq {
		seq = remoteSeq
	}
	seq++

	b := &bundle.Bundle{
		Meta: bundle.Meta{
			ProjectID: projectID,
			Token:     token,
			Baton:     bundle.Baton{Owner: "", Sequence: seq, UpdatedAt: now},
		},
		Sessions: neutral,
	}
	if err := d.Transport.Send(b); err != nil {
		return Report{}, err
	}

	st.ProjectID = projectID
	st.Machine = d.Cfg.Machine
	st.LastSyncedSequence = seq
	st.BatonHeld = false
	st.LastSyncAt = now
	st.Sessions = snapshot(sessions)
	if err := st.Save(d.statePath(projectID)); err != nil {
		return Report{}, err
	}
	return Report{ProjectID: projectID, Sessions: len(sessions), Sequence: seq}, nil
}

/*
ErrStalePull means the baton still names another machine that never handed
off; pulling risks discarding that machine's newer work.
*/
var ErrStalePull = errors.New("hop: another machine still holds the baton (use force to override)")

/*
Pull receives the bundle, materializes each incoming session to the local root,
merges per session (consulting r on a genuine fork), claims the baton locally,
and refreshes state. The global sequence+dirty condition is a non-fatal notice;
per-session merge plus r is the sole divergence authority.
*/
func Pull(d Deps, projectID, now string, r Resolver) (Report, error) {
	root, err := d.localRoot(projectID)
	if err != nil {
		return Report{}, err
	}
	b, err := d.Transport.Receive(projectID)
	if err != nil {
		return Report{}, err
	}
	st, err := state.Load(d.statePath(projectID))
	if err != nil {
		return Report{}, err
	}
	local, err := d.Agent.ListSessions(d.Home, root)
	if err != nil {
		return Report{}, err
	}

	// Stale-pull: only a forcing resolver bypasses it.
	if !forcesStalePull(r) && b.Meta.Baton.Owner != "" && b.Meta.Baton.Owner != d.Cfg.Machine {
		return Report{}, ErrStalePull
	}
	// Both machines advanced: a non-fatal heads-up, not an abort. Per-session
	// merge below is the real authority.
	if b.Meta.Baton.Sequence != st.LastSyncedSequence && st.Dirty(local) && d.Notify != nil {
		d.Notify("sync conflict: both machines advanced since the last sync — checking each session")
	}

	localByID := map[string][]byte{}
	for _, s := range local {
		localByID[s.ID] = s.Data
	}

	written := 0
	for _, in := range b.Sessions {
		materialized, err := rewrite.Materialize(in.Data, root, d.OS, b.Meta.Token)
		if err != nil {
			return Report{}, err
		}
		switch merge.Decide(localByID[in.ID], materialized) {
		case merge.New, merge.Update:
			if err := d.Agent.WriteSession(d.Home, root, agent.Session{ID: in.ID, Data: materialized}); err != nil {
				return Report{}, err
			}
			written++
		case merge.Diverged:
			res, err := r.Resolve(in.ID, localByID[in.ID], materialized)
			if err != nil {
				return Report{}, err
			}
			switch res {
			case KeepIncoming:
				if _, err := d.Agent.BackupSession(d.Home, root, in.ID); err != nil {
					return Report{}, err
				}
				if err := d.Agent.WriteSession(d.Home, root, agent.Session{ID: in.ID, Data: materialized}); err != nil {
					return Report{}, err
				}
				written++
			case KeepLocal:
				// Leave the local session untouched.
			case Abort:
				return Report{}, fmt.Errorf("%w: session %s", ErrDiverged, in.ID)
			}
		case merge.NoOp, merge.KeepLocalNewer, merge.KeepLocalOnly:
			// Nothing to write.
		}
	}

	// Claim the baton locally and record this sync.
	st.ProjectID = projectID
	st.Machine = d.Cfg.Machine
	st.LastSyncedSequence = b.Meta.Baton.Sequence
	st.BatonHeld = true
	st.LastSyncAt = now
	// Snapshot reflects what is now on disk (re-list to include untouched ones).
	after, err := d.Agent.ListSessions(d.Home, root)
	if err != nil {
		return Report{}, err
	}
	st.Sessions = snapshot(after)
	if err := st.Save(d.statePath(projectID)); err != nil {
		return Report{}, err
	}
	return Report{ProjectID: projectID, Sessions: written, Sequence: b.Meta.Baton.Sequence}, nil
}

/* snapshot builds a state snapshot of every session's size and hash. */
func snapshot(sessions []agent.Session) map[string]state.SessionSnap {
	m := make(map[string]state.SessionSnap, len(sessions))
	for _, s := range sessions {
		m[s.ID] = state.Snap(s.Data)
	}
	return m
}
