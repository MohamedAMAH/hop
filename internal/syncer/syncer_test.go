package syncer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"hop/internal/agent"
	"hop/internal/agent/claude"
	"hop/internal/bundle"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/transport/fake"
)

func agentSession(id string, data []byte) agent.Session { return agent.Session{ID: id, Data: data} }

func writeSession(t *testing.T, c claude.Claude, home, root, id, data string) {
	t.Helper()
	if err := c.WriteSession(home, root, agentSession(id, []byte(data))); err != nil {
		t.Fatal(err)
	}
}

func baseDeps(t *testing.T) (Deps, string) {
	t.Helper()
	home := t.TempDir()
	root := home + "/proj"
	cfg := config.Config{Machine: "win", Projects: map[string]config.Project{
		"hop": {Paths: map[string]string{"win": root}, Transport: "folder", Handoff: "manual"}}}
	return Deps{
		Cfg: cfg, Agent: claude.New(), Transport: fake.New(),
		Home: home, StateDir: t.TempDir(), OS: osinfo.Unix,
	}, root
}

func TestPushNeutralizesAndBumpsSequence(t *testing.T) {
	d, root := baseDeps(t)
	// root may contain backslashes (Windows temp dirs), so it must be
	// JSON-encoded rather than concatenated raw into the fixture string.
	cwd, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	writeSession(t, claude.New(), d.Home, root, "s1",
		`{"cwd":`+string(cwd)+`,"x":1}`+"\n")

	rep, err := Push(d, "hop", "2026-07-06T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Sessions != 1 || rep.Sequence != 1 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	got, err := d.Transport.Receive("hop")
	if err != nil {
		t.Fatal(err)
	}
	// The pushed bundle must be neutralized (no raw root) and baton released.
	if string(got.Sessions[0].Data) != `{"cwd":"__HOP_ROOT__","x":1}`+"\n" {
		t.Fatalf("not neutralized: %q", got.Sessions[0].Data)
	}
	if got.Meta.Baton.Owner != "" || got.Meta.Baton.Sequence != 1 {
		t.Fatalf("baton not released/bumped: %+v", got.Meta.Baton)
	}
}

func localDirtySession(t *testing.T, d Deps, root string) {
	t.Helper()
	cwd, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	writeSession(t, claude.New(), d.Home, root, "s1", `{"cwd":`+string(cwd)+`,"x":1}`+"\n")
}

// A fresh destination reports a bumped sequence but holds no sessions; pushing
// local work onto it must not be mistaken for divergence.
func TestPushToEmptyRemoteDoesNotDiverge(t *testing.T) {
	d, root := baseDeps(t)
	localDirtySession(t, d, root)
	if err := d.Transport.Send(&bundle.Bundle{
		Meta: bundle.Meta{ProjectID: "hop", Baton: bundle.Baton{Sequence: 5}},
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := Push(d, "hop", "2026-07-12T00:00:00Z")
	if err != nil {
		t.Fatalf("push onto empty remote must not diverge: %v", err)
	}
	if rep.Sessions != 1 || rep.Sequence != 6 {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

// When the remote actually holds sessions at a sequence we never synced and we
// also have local changes, both machines moved and the push must abort.
func TestPushDivergesWhenRemoteHasSessions(t *testing.T) {
	d, root := baseDeps(t)
	localDirtySession(t, d, root)
	if err := d.Transport.Send(&bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Baton: bundle.Baton{Sequence: 5}},
		Sessions: []agent.Session{agentSession("s2", []byte(`{"cwd":"__HOP_ROOT__","y":2}`+"\n"))},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(d, "hop", "2026-07-12T00:00:00Z"); !errors.Is(err, ErrDiverged) {
		t.Fatalf("push must diverge when remote holds unsynced sessions, got %v", err)
	}
}

func TestPullMaterializesNewSession(t *testing.T) {
	// Producer machine "win" pushes; consumer "nix" pulls to a different root.
	prod, prodRoot := baseDeps(t)
	prodCwd, err := json.Marshal(prodRoot)
	if err != nil {
		t.Fatal(err)
	}
	writeSession(t, claude.New(), prod.Home, prodRoot, "s1",
		`{"cwd":`+string(prodCwd)+`,"x":1}`+"\n")
	if _, err := Push(prod, "hop", "2026-07-06T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	// Consumer shares the SAME transport but a different home/root/machine.
	consHome := t.TempDir()
	consRoot := consHome + "/elsewhere/hop"
	cons := Deps{
		Cfg: config.Config{Machine: "nix", Projects: map[string]config.Project{
			"hop": {Paths: map[string]string{"nix": consRoot}, Transport: "folder"}}},
		Agent: claude.New(), Transport: prod.Transport,
		Home: consHome, StateDir: t.TempDir(), OS: osinfo.Unix,
	}

	rep, err := Pull(cons, "hop", "2026-07-06T01:00:00Z", AbortResolver{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Sessions != 1 {
		t.Fatalf("expected 1 session, got %+v", rep)
	}
	got, err := claude.New().ListSessions(consHome, consRoot)
	if err != nil {
		t.Fatal(err)
	}
	// cwd must be rewritten to the consumer's root.
	consCwd, err := json.Marshal(consRoot)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cwd":` + string(consCwd) + `,"x":1}` + "\n"
	if len(got) != 1 || string(got[0].Data) != want {
		t.Fatalf("materialize mismatch:\n got  %q\n want %q", got[0].Data, want)
	}
}

/*
divergedSetup returns a producer Deps+root, a consumer Deps+root sharing one
transport, and the local-divergent session bytes it wrote for the consumer,
where the consumer already holds a session with the same ID as the incoming
one but byte-forked content, forcing merge.Diverged.
*/
func divergedSetup(t *testing.T) (Deps, string, Deps, string, []byte) {
	t.Helper()
	// Producer machine "win" pushes; consumer "nix" has an independently
	// diverged local copy of the same session ID.
	prod, prodRoot := baseDeps(t)
	prodCwd, err := json.Marshal(prodRoot)
	if err != nil {
		t.Fatal(err)
	}
	writeSession(t, claude.New(), prod.Home, prodRoot, "s1",
		`{"cwd":`+string(prodCwd)+`,"x":1}`+"\n")
	if _, err := Push(prod, "hop", "2026-07-06T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	consHome := t.TempDir()
	consRoot := consHome + "/elsewhere/hop"
	cons := Deps{
		Cfg: config.Config{Machine: "nix", Projects: map[string]config.Project{
			"hop": {Paths: map[string]string{"nix": consRoot}, Transport: "folder"}}},
		Agent: claude.New(), Transport: prod.Transport,
		Home: consHome, StateDir: t.TempDir(), OS: osinfo.Unix,
	}
	// Local content that shares neither a prefix nor a suffix relationship
	// with the incoming bytes, forcing merge.Diverged.
	localData := `{"cwd":"/local/only","y":999}` + "\n"
	writeSession(t, claude.New(), cons.Home, consRoot, "s1", localData)

	return prod, prodRoot, cons, consRoot, []byte(localData)
}

func TestForcedPullBacksUpDivergedSession(t *testing.T) {
	_, _, cons, consRoot, localData := divergedSetup(t)
	consHome := cons.Home

	if _, err := Pull(cons, "hop", "2026-07-06T01:00:00Z", ForceResolver{}); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(claude.New().ProjectDir(consHome, consRoot), ".hop-backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("expected backup dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 backup file, got %d", len(entries))
	}
	backupBytes, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(backupBytes) != string(localData) {
		t.Fatalf("backup bytes = %q, want %q", backupBytes, localData)
	}

	// Assert that the live session was overwritten with the incoming (materialized) content.
	got, err := claude.New().ListSessions(cons.Home, consRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 session after forced pull, got %d", len(got))
	}
	// The materialized incoming content should have the consumer's root, not the producer's.
	consCwd, err := json.Marshal(consRoot)
	if err != nil {
		t.Fatal(err)
	}
	expectedData := `{"cwd":` + string(consCwd) + `,"x":1}` + "\n"
	if string(got[0].Data) != expectedData {
		t.Fatalf("overwritten session data mismatch:\n got  %q\n want %q", got[0].Data, expectedData)
	}
}

/* scriptedResolver returns a fixed Resolution and records that it was called. */
type scriptedResolver struct {
	res    Resolution
	called bool
}

func (s *scriptedResolver) Resolve(string, []byte, []byte) (Resolution, error) {
	s.called = true
	return s.res, nil
}

func TestPullKeepLocalSkipsWrite(t *testing.T) {
	d, prodRoot, cons, consRoot, _ := divergedSetup(t)
	_ = prodRoot
	before, _ := claude.New().ListSessions(cons.Home, consRoot)
	r := &scriptedResolver{res: KeepLocal}
	if _, err := Pull(cons, "hop", "2026-07-08T00:00:00Z", r); err != nil {
		t.Fatal(err)
	}
	if !r.called {
		t.Fatal("resolver was not consulted on divergence")
	}
	after, _ := claude.New().ListSessions(cons.Home, consRoot)
	if string(after[0].Data) != string(before[0].Data) {
		t.Fatal("KeepLocal must not overwrite the local session")
	}
	_ = d
}

func TestPullAbortReturnsErrDiverged(t *testing.T) {
	_, _, cons, _, _ := divergedSetup(t)
	if _, err := Pull(cons, "hop", "2026-07-08T00:00:00Z", AbortResolver{}); !errors.Is(err, ErrDiverged) {
		t.Fatalf("AbortResolver must yield ErrDiverged, got %v", err)
	}
}

func TestBothAdvancedIsNoticeNotAbort(t *testing.T) {
	// Machine A pushes session X; machine B has a NEW local session Y (no shared
	// fork). Both advanced, but there is no per-session conflict, so pull must
	// succeed under AbortResolver and fire the notice.
	d, root := baseDeps(t)
	cwd, _ := json.Marshal(root)
	writeSession(t, claude.New(), d.Home, root, "x", `{"cwd":`+string(cwd)+`,"n":1}`+"\n")
	if _, err := Push(d, "hop", "2026-07-08T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// Consumer with its own new local session Y and a stale last-synced sequence.
	consHome := t.TempDir()
	consRoot := consHome + "/proj"
	ccwd, _ := json.Marshal(consRoot)
	cons := Deps{
		Cfg:   config.Config{Machine: "B", Projects: map[string]config.Project{"hop": {Paths: map[string]string{"B": consRoot}}}},
		Agent: claude.New(), Transport: d.Transport, Home: consHome, StateDir: t.TempDir(), OS: osinfo.Unix,
	}
	writeSession(t, claude.New(), consHome, consRoot, "y", `{"cwd":`+string(ccwd)+`,"m":2}`+"\n")
	notes := 0
	cons.Notify = func(string) { notes++ }
	rep, err := Pull(cons, "hop", "2026-07-08T01:00:00Z", AbortResolver{})
	if err != nil {
		t.Fatalf("both-advanced with no fork must not abort: %v", err)
	}
	if notes == 0 {
		t.Fatal("expected the both-advanced notice to fire")
	}
	// Y survived, X arrived.
	got, _ := claude.New().ListSessions(consHome, consRoot)
	if len(got) != 2 {
		t.Fatalf("expected both sessions present, got %d", len(got))
	}
	_ = rep
}
