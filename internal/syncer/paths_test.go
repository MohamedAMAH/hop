package syncer

import (
	"strings"
	"testing"

	"hop/internal/osinfo"
)

func TestSafeArtifactPathRejectsTraversal(t *testing.T) {
	bad := []string{"../evil", "sess/../../evil", "/abs/path", `C:\win`, "a/../../b", ".."}
	for _, p := range bad {
		if err := safeArtifactPath(p); err == nil {
			t.Fatalf("expected %q to be rejected", p)
		}
	}
	good := []string{"sess/subagents/a.jsonl", "memory/MEMORY.md", "tool-results/x.txt"}
	for _, p := range good {
		if err := safeArtifactPath(p); err != nil {
			t.Fatalf("expected %q to be allowed, got %v", p, err)
		}
	}
}

func TestNeutralizeMaterializeBothRoots(t *testing.T) {
	// A subagent line embeds both the project root and the storage prefix.
	src := []byte(`{"cwd":"d:\\proj","ref":"d:\\store\\tool-results\\x.txt"}` + "\n")
	root, prefix := "d:\\proj", "d:\\store"
	neut, err := neutralizeAll(src, root, prefix, osinfo.Windows, "__R__", "__S__")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(neut), "d:\\proj") || strings.Contains(string(neut), "d:\\store") {
		t.Fatalf("roots not neutralized: %s", neut)
	}
	// Materialize onto a different machine (unix paths).
	mat, err := materializeAll(neut, "/home/u/proj", "/home/u/store", osinfo.Unix, "__R__", "__S__")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mat), "/home/u/proj") || !strings.Contains(string(mat), "/home/u/store/tool-results/x.txt") {
		t.Fatalf("not materialized to target: %s", mat)
	}
}
