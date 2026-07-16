package claude

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"hop/internal/agent"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListArtifactsSkipsTranscriptsAndBackups(t *testing.T) {
	home := t.TempDir()
	root := "d:/proj"
	dir := Claude{}.ProjectDir(home, root)
	writeFile(t, filepath.Join(dir, "sess.jsonl"), "transcript\n")            // excluded (top-level jsonl)
	writeFile(t, filepath.Join(dir, "sess", "subagents", "a.jsonl"), "sub\n") // sidecar
	writeFile(t, filepath.Join(dir, "sess", "tool-results", "t.txt"), "blob") // sidecar
	writeFile(t, filepath.Join(dir, "memory", "MEMORY.md"), "mem")            // memory
	writeFile(t, filepath.Join(dir, ".hop-backups", "b.jsonl"), "bak")        // excluded

	arts, err := Claude{}.ListArtifacts(home, root)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, a := range arts {
		got = append(got, a.RelPath)
	}
	sort.Strings(got)
	want := []string{"memory/MEMORY.md", "sess/subagents/a.jsonl", "sess/tool-results/t.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	if (Claude{}).Classify("memory/MEMORY.md") != agent.KindMemory {
		t.Fatal("memory/ must be KindMemory")
	}
	if (Claude{}).Classify("sess/subagents/a.jsonl") != agent.KindSidecar {
		t.Fatal("sidecar must be KindSidecar")
	}
}

func TestRewritesPaths(t *testing.T) {
	if !(Claude{}).RewritesPaths("s1/subagents/a.jsonl") {
		t.Fatal("a sidecar .jsonl must report RewritesPaths true")
	}
	if (Claude{}).RewritesPaths("memory/notes.jsonl") {
		t.Fatal("a memory .jsonl must report RewritesPaths false")
	}
	if (Claude{}).RewritesPaths("s1/tool-results/x.txt") {
		t.Fatal("a non-jsonl sidecar must report RewritesPaths false")
	}
	if (Claude{}).RewritesPaths("memory/MEMORY.md") {
		t.Fatal("a non-jsonl memory file must report RewritesPaths false")
	}
}

func TestWriteAndReadArtifactRoundTrip(t *testing.T) {
	home := t.TempDir()
	root := "d:/proj"
	a := agent.Artifact{RelPath: "sess/subagents/a.jsonl", Data: []byte("hello")}
	if err := (Claude{}).WriteArtifact(home, root, a); err != nil {
		t.Fatal(err)
	}
	data, _, exists, err := Claude{}.ReadArtifact(home, root, "sess/subagents/a.jsonl")
	if err != nil || !exists || string(data) != "hello" {
		t.Fatalf("read back = %q exists=%v err=%v", data, exists, err)
	}
	_, _, exists, err = Claude{}.ReadArtifact(home, root, "sess/missing.txt")
	if err != nil || exists {
		t.Fatalf("missing artifact should report exists=false, got exists=%v err=%v", exists, err)
	}
}

func TestWriteArtifactPreservesModTime(t *testing.T) {
	home := t.TempDir()
	root := "d:/proj"
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).UnixNano()
	a := agent.Artifact{RelPath: "memory/MEMORY.md", Data: []byte("note"), ModTime: want}
	if err := (Claude{}).WriteArtifact(home, root, a); err != nil {
		t.Fatal(err)
	}
	_, got, exists, err := Claude{}.ReadArtifact(home, root, "memory/MEMORY.md")
	if err != nil || !exists {
		t.Fatalf("read back exists=%v err=%v", exists, err)
	}
	// Compared at second granularity, since some filesystems do not preserve sub-second mod times.
	if got/1e9 != want/1e9 {
		t.Fatalf("mod time not preserved: got %d, want %d", got, want)
	}
}
