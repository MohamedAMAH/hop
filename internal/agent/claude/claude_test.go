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

func TestBackupSessionMissingReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	root := `/proj/x`
	c := New()
	path, err := c.BackupSession(home, root, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("BackupSession path = %q, want empty", path)
	}
}

func TestBackupSessionCopiesExistingBytes(t *testing.T) {
	home := t.TempDir()
	root := `/proj/x`
	c := New()
	in := []byte(`{"type":"user"}` + "\n")
	if err := c.WriteSession(home, root, agent.Session{ID: "abc", Data: in}); err != nil {
		t.Fatal(err)
	}
	path, err := c.BackupSession(home, root, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("BackupSession path = \"\", want a backup path")
	}
	backupDir := filepath.Join(c.ProjectDir(home, root), ".hop-backups")
	if filepath.Dir(path) != backupDir {
		t.Fatalf("backup path %q not under %q", path, backupDir)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(in) {
		t.Fatalf("backup bytes = %q, want %q", got, in)
	}
	// Confirm the backup directory is not seen as a session by ListSessions.
	sessions, err := c.ListSessions(home, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "abc" {
		t.Fatalf("ListSessions should only see the real session, got %+v", sessions)
	}
}
