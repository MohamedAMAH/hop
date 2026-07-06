package claude

import (
	"os"
	"path/filepath"
	"testing"

	"hop/internal/agent"
)

func TestProjectDir(t *testing.T) {
	c := New()
	got := c.ProjectDir("/home/abdel", "/home/abdel/hop")
	want := filepath.Join("/home/abdel", ".claude", "projects", "-home-abdel-hop")
	if got != want {
		t.Fatalf("ProjectDir = %q, want %q", got, want)
	}
}

func TestWriteThenListRoundTrips(t *testing.T) {
	home := t.TempDir()
	root := `/proj/x`
	c := New()
	in := []byte(`{"type":"user"}` + "\n")
	if err := c.WriteSession(home, root, agent.Session{ID: "abc", Data: in}); err != nil {
		t.Fatal(err)
	}
	got, err := c.ListSessions(home, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "abc" || string(got[0].Data) != string(in) {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	// Confirm it landed in the encoded dir.
	if _, err := os.Stat(filepath.Join(c.ProjectDir(home, root), "abc.jsonl")); err != nil {
		t.Fatalf("expected session file: %v", err)
	}
}
