package lan

import (
	"bytes"
	"testing"

	"hop/internal/agent"
	"hop/internal/bundle"
)

func TestWireRoundTrip(t *testing.T) {
	b := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "demo", Token: bundle.DefaultToken, Baton: bundle.Baton{Sequence: 3}},
		Sessions: []agent.Session{{ID: "s1", Data: []byte(`{"x":1}` + "\n")}},
	}
	data, err := encodeBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.Baton.Sequence != 3 || len(got.Sessions) != 1 || got.Sessions[0].ID != "s1" ||
		!bytes.Equal(got.Sessions[0].Data, b.Sessions[0].Data) {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestWireRoundTripsFiles(t *testing.T) {
	in := &bundle.Bundle{
		Meta:     bundle.Meta{ProjectID: "hop", Token: "__R__", PrefixToken: "__S__", Baton: bundle.Baton{Sequence: 2}},
		Sessions: []agent.Session{{ID: "s1", Data: []byte("hi\n")}},
		Files:    []bundle.FileEntry{{Path: "memory/MEMORY.md", Data: []byte("m"), Hash: bundle.HashBytes([]byte("m")), ModTime: 5}},
	}
	enc, err := encodeBundle(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeBundle(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 1 || string(out.Files[0].Data) != "m" || out.Files[0].ModTime != 5 {
		t.Fatalf("file not round-tripped: %+v", out.Files)
	}
	if out.Meta.PrefixToken != "__S__" {
		t.Fatalf("prefix token lost: %q", out.Meta.PrefixToken)
	}
}

func TestWireRejectsTamper(t *testing.T) {
	b := &bundle.Bundle{Meta: bundle.Meta{ProjectID: "demo"}, Sessions: []agent.Session{{ID: "s1", Data: []byte("aaa")}}}
	data, _ := encodeBundle(b)
	// Corrupt the base64-encoded data value to cause a hash mismatch.
	corrupt := bytes.Replace(data, []byte("YWFh"), []byte("YWYY"), 1)
	if _, err := decodeBundle(corrupt); err == nil {
		t.Fatal("expected a hash-mismatch error on tampered payload")
	}
}

func TestWireDecodeRejectsUnsafeFilePath(t *testing.T) {
	b := &bundle.Bundle{
		Meta:  bundle.Meta{ProjectID: "demo"},
		Files: []bundle.FileEntry{{Path: "../escape.txt", Data: []byte("x"), Hash: bundle.HashBytes([]byte("x"))}},
	}
	data, err := encodeBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBundle(data); err == nil {
		t.Fatal("expected decodeBundle to reject an unsafe file path")
	}
}
