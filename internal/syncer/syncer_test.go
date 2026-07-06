package syncer

import (
	"encoding/json"
	"testing"

	"hop/internal/agent"
	"hop/internal/agent/claude"
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
