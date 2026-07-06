/*
Package state stores hop's machine-private per-project sync memory, used to
detect unpushed local work (the "dirty" check) and sequence divergence.
*/
package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"hop/internal/agent"
)

/* SessionSnap records one session's size and content hash at last sync. */
type SessionSnap struct {
	Bytes int64  `json:"bytes"`
	Hash  string `json:"hash"`
}

/* State is the local state file for a single project. */
type State struct {
	ProjectID          string                 `json:"projectId"`
	Machine            string                 `json:"machine"`
	LastSyncedSequence int                    `json:"lastSyncedSequence"`
	BatonHeld          bool                   `json:"batonHeld"`
	LastSyncAt         string                 `json:"lastSyncAt"`
	Sessions           map[string]SessionSnap `json:"sessions"`
}

/* Snap computes the snapshot (length + sha256) of a session's bytes. */
func Snap(data []byte) SessionSnap {
	sum := sha256.Sum256(data)
	return SessionSnap{Bytes: int64(len(data)), Hash: fmt.Sprintf("sha256:%x", sum)}
}

/*
Dirty reports whether the current sessions differ from the snapshot: any new
session, any differing length, any differing hash, or any deleted session.
*/
func (s State) Dirty(sessions []agent.Session) bool {
	currentIDs := make(map[string]bool)
	for _, sess := range sessions {
		currentIDs[sess.ID] = true
		snap, ok := s.Sessions[sess.ID]
		if !ok {
			return true
		}
		if snap.Bytes != int64(len(sess.Data)) {
			return true
		}
		if snap.Hash != Snap(sess.Data).Hash {
			return true
		}
	}
	// Check for deleted sessions: snapshot IDs not in current set.
	for snapID := range s.Sessions {
		if !currentIDs[snapID] {
			return true
		}
	}
	return false
}

/*
Load reads the state file; a missing file yields a zero State with an
initialized Sessions map (not an error).
*/
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{Sessions: map[string]SessionSnap{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	if s.Sessions == nil {
		s.Sessions = map[string]SessionSnap{}
	}
	return s, nil
}

/* Save writes the state file atomically (temp + rename). */
func (s State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
