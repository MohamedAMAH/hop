package syncer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hop/internal/agent"
	"hop/internal/agent/claude"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/transport/fake"
)

func TestEndToEndCrossRoot(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture uses /src/proj as the root; rewrite it to each machine's root.
	prodHome := t.TempDir()
	prodRoot := "/src/proj"
	// Place the fixture as-is (its cwd already equals prodRoot).
	if err := claude.New().WriteSession(prodHome, prodRoot,
		agentSession("5038b5e4", fixture)); err != nil {
		t.Fatal(err)
	}
	shared := fake.New()
	prod := Deps{
		Cfg:   config.Config{Machine: "A", Projects: map[string]config.Project{"demo": {Paths: map[string]string{"A": prodRoot}}}},
		Agent: claude.New(), Transport: shared, Home: prodHome, StateDir: t.TempDir(), OS: osinfo.Unix,
	}
	if _, err := Push(prod, "demo", "2026-07-06T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	consHome := t.TempDir()
	consRoot := "/home/b/demo"
	cons := Deps{
		Cfg:   config.Config{Machine: "B", Projects: map[string]config.Project{"demo": {Paths: map[string]string{"B": consRoot}}}},
		Agent: claude.New(), Transport: shared, Home: consHome, StateDir: t.TempDir(), OS: osinfo.Unix,
	}
	if _, err := Pull(cons, "demo", "2026-07-06T01:00:00Z", AbortResolver{}); err != nil {
		t.Fatal(err)
	}

	got, err := claude.New().ListSessions(consHome, consRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 session, got %d", len(got))
	}
	text := string(got[0].Data)
	if strings.Contains(text, "/src/proj") {
		t.Fatalf("producer root leaked into materialized session:\n%s", text)
	}
	if !strings.Contains(text, `"cwd":"/home/b/demo"`) {
		t.Fatalf("cwd not rewritten to consumer root:\n%s", text)
	}
	if !strings.Contains(text, "edit /home/b/demo/main.go please") {
		t.Fatalf("embedded free-form path not rewritten:\n%s", text)
	}
}

func TestFullSessionSyncResolvesSidecarReference(t *testing.T) {
	// Machine A.
	a, aRoot := baseDeps(t)
	localDirtySession(t, a, aRoot)
	aStore := claude.New().ProjectDir(a.Home, aRoot)
	ref := aStore + "/s1/tool-results/x.txt"
	if err := claude.New().WriteArtifact(a.Home, aRoot,
		agent.Artifact{RelPath: "s1/subagents/agent-1.jsonl", Data: []byte(`{"ref":` + jsonStr(ref) + "}\n")}); err != nil {
		t.Fatal(err)
	}
	if err := claude.New().WriteArtifact(a.Home, aRoot,
		agent.Artifact{RelPath: "s1/tool-results/x.txt", Data: []byte("payload-bytes")}); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(a, "hop", "2026-07-16T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	// Machine B: different home, different project root, shared transport.
	b, bRoot := baseDeps(t)
	b.Transport = a.Transport
	if _, err := Pull(b, "hop", "2026-07-16T00:01:00Z", AbortResolver{}); err != nil {
		t.Fatal(err)
	}
	bStore := claude.New().ProjectDir(b.Home, bRoot)
	sub, _, ok, _ := claude.New().ReadArtifact(b.Home, bRoot, "s1/subagents/agent-1.jsonl")
	if !ok {
		t.Fatal("subagent transcript missing on B")
	}
	// Decode rather than compare raw bytes: a store dir may contain
	// backslashes (Windows), which the JSON encoding escapes as "\\".
	var decoded struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(sub), &decoded); err != nil {
		t.Fatalf("subagent transcript is not valid JSON: %v\n%s", err, sub)
	}
	if decoded.Ref != bStore+"/s1/tool-results/x.txt" {
		t.Fatalf("reference not re-homed to B: %q", decoded.Ref)
	}
	if strings.Contains(decoded.Ref, aStore) {
		t.Fatalf("reference still points at A: %q", decoded.Ref)
	}
	payload, _, ok, _ := claude.New().ReadArtifact(b.Home, bRoot, "s1/tool-results/x.txt")
	if !ok || string(payload) != "payload-bytes" {
		t.Fatalf("payload not identical on B: %q ok=%v", payload, ok)
	}
}
