package folder

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"hop/internal/agent"
	"hop/internal/bundle"
	"hop/internal/transport"
)

func TestFolderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrNoBundle) {
		t.Fatalf("empty Receive must return ErrNoBundle, got %v", err)
	}
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken, Baton: bundle.Baton{Owner: "win", Sequence: 2}},
		Sessions: []agent.Session{{ID: "abc", Data: []byte(`{"type":"user"}` + "\n")}},
	}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	got, err := f.Receive("hop")
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Baton.Sequence != 2 || len(got.Sessions) != 1 || got.Sessions[0].ID != "abc" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if string(got.Sessions[0].Data) != `{"type":"user"}`+"\n" {
		t.Fatalf("session data mismatch: %q", got.Sessions[0].Data)
	}
}

func TestFolderRoundTripsFiles(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	in := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Baton: bundle.Baton{Sequence: 1}},
		Sessions: []agent.Session{{ID: "s1", Data: []byte("hi\n")}},
		Files: []bundle.FileEntry{
			{Path: "s1/subagents/a.jsonl", Data: []byte("sub"), Hash: bundle.HashBytes([]byte("sub")), ModTime: 7},
			{Path: "memory/MEMORY.md", Data: []byte("m"), Hash: bundle.HashBytes([]byte("m")), ModTime: 9},
		},
	}
	if err := f.Send(in); err != nil {
		t.Fatal(err)
	}
	out, err := f.Receive("hop")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(out.Files))
	}
	byPath := map[string]bundle.FileEntry{}
	for _, e := range out.Files {
		byPath[e.Path] = e
	}
	if string(byPath["memory/MEMORY.md"].Data) != "m" || byPath["memory/MEMORY.md"].ModTime != 9 {
		t.Fatalf("memory file not round-tripped: %+v", byPath["memory/MEMORY.md"])
	}
}

func TestFolderReceiveEmptyDirReturnsErrNoBundle(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrNoBundle) {
		t.Fatalf("empty dir Receive must return ErrNoBundle, got %v", err)
	}
}

func TestFolderReceiveIgnoresStaleSessionFileNotInManifest(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken, Baton: bundle.Baton{Owner: "win", Sequence: 1}},
		Sessions: []agent.Session{{ID: "abc", Data: []byte(`{"type":"user"}` + "\n")}},
	}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	// Simulate a leftover session file from an older push that is not part of this bundle's manifest.
	stalePath := filepath.Join(dir, "hop", "stale.jsonl")
	if err := os.WriteFile(stalePath, []byte(`{"type":"stale"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := f.Receive("hop")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].ID != "abc" {
		t.Fatalf("stale file must be ignored, got sessions: %+v", got.Sessions)
	}
}

func TestFolderReceiveReturnsErrIncompleteBundleWhenSessionFileMissing(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken, Baton: bundle.Baton{Owner: "win", Sequence: 1}},
		Sessions: []agent.Session{{ID: "abc", Data: []byte(`{"type":"user"}` + "\n")}},
	}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "hop", "abc.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrIncompleteBundle) {
		t.Fatalf("missing session file must return ErrIncompleteBundle, got %v", err)
	}
}

func TestFolderReceiveReturnsErrIncompleteBundleWhenSessionFileCorrupted(t *testing.T) {
	dir := t.TempDir()
	f := New(dir)
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken, Baton: bundle.Baton{Owner: "win", Sequence: 1}},
		Sessions: []agent.Session{{ID: "abc", Data: []byte(`{"type":"user"}` + "\n")}},
	}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hop", "abc.jsonl"), []byte(`{"type":"tampered"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrIncompleteBundle) {
		t.Fatalf("corrupted session file must return ErrIncompleteBundle, got %v", err)
	}
}

func TestFolderReceiveReturnsErrIncompleteBundleWhenSessionFileHashMismatch(t *testing.T) {
	// This test ensures the hash-check branch is covered by tampering with
	// a file that has the same byte length but different content, so the
	// length check passes but the sha256 check fails.
	dir := t.TempDir()
	f := New(dir)
	originalData := []byte(`{"type":"user"}` + "\n")
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken, Baton: bundle.Baton{Owner: "win", Sequence: 1}},
		Sessions: []agent.Session{{ID: "abc", Data: originalData}},
	}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	// Build a same-length replacement by creating identical-length bytes.
	// Original is {"type":"user"}\n (16 bytes); use {"type":"admin"}\n (also 16 bytes).
	sameLengthTamper := make([]byte, len(originalData))
	copy(sameLengthTamper, []byte(`{"type":"admin"}`+"\n"))
	if len(sameLengthTamper) != len(originalData) {
		t.Fatalf("tamper length mismatch: %d != %d", len(sameLengthTamper), len(originalData))
	}
	if err := os.WriteFile(filepath.Join(dir, "hop", "abc.jsonl"), sameLengthTamper, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrIncompleteBundle) {
		t.Fatalf("hash mismatch must return ErrIncompleteBundle, got %v", err)
	}
}
