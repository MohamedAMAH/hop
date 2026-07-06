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
}

/* Report summarizes an operation for display. */
type Report struct {
	ProjectID string
	Sessions  int
	Sequence  int
	Note      string
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
	// have local changes, both moved.
	remoteSeq := 0
	if rb, rerr := d.Transport.Receive(projectID); rerr == nil {
		remoteSeq = rb.Meta.Baton.Sequence
	} else if !errors.Is(rerr, transport.ErrNoBundle) {
		return Report{}, rerr
	}
	if remoteSeq != st.LastSyncedSequence && st.Dirty(sessions) {
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

/* snapshot builds a state snapshot of every session's size and hash. */
func snapshot(sessions []agent.Session) map[string]state.SessionSnap {
	m := make(map[string]state.SessionSnap, len(sessions))
	for _, s := range sessions {
		m[s.ID] = state.Snap(s.Data)
	}
	return m
}
