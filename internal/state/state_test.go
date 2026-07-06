package state

import (
	"path/filepath"
	"testing"

	"hop/internal/agent"
)

func snapshotOf(sessions []agent.Session) map[string]SessionSnap {
	m := map[string]SessionSnap{}
	for _, s := range sessions {
		m[s.ID] = Snap(s.Data)
	}
	return m
}

func TestDirty(t *testing.T) {
	base := []agent.Session{{ID: "a", Data: []byte("hello")}}
	st := State{Sessions: snapshotOf(base)}

	if st.Dirty(base) {
		t.Fatal("unchanged sessions must be clean")
	}
	// New session with no snapshot entry.
	if !st.Dirty([]agent.Session{{ID: "a", Data: []byte("hello")}, {ID: "b", Data: []byte("x")}}) {
		t.Fatal("a new session must be dirty")
	}
	// Grown session (append-only).
	if !st.Dirty([]agent.Session{{ID: "a", Data: []byte("hello!")}}) {
		t.Fatal("a grown session must be dirty")
	}
	// Same length, different content.
	if !st.Dirty([]agent.Session{{ID: "a", Data: []byte("world")}}) {
		t.Fatal("same-length different-content must be dirty")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hop.json")
	st := State{ProjectID: "hop", Machine: "m1", LastSyncedSequence: 3,
		Sessions: map[string]SessionSnap{"a": Snap([]byte("x"))}}
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSyncedSequence != 3 || got.Sessions["a"] != st.Sessions["a"] {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestLoadMissingReturnsZeroState(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing state must not error: %v", err)
	}
	if got.Sessions == nil {
		t.Fatal("missing state must return an initialized Sessions map")
	}
}
