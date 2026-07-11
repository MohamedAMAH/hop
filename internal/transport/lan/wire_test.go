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

func TestWireRejectsTamper(t *testing.T) {
	b := &bundle.Bundle{Meta: bundle.Meta{ProjectID: "demo"}, Sessions: []agent.Session{{ID: "s1", Data: []byte("aaa")}}}
	data, _ := encodeBundle(b)
	// Corrupt the base64-encoded data value to cause a hash mismatch.
	corrupt := bytes.Replace(data, []byte("YWFh"), []byte("YWYY"), 1)
	if _, err := decodeBundle(corrupt); err == nil {
		t.Fatal("expected a hash-mismatch error on tampered payload")
	}
}
