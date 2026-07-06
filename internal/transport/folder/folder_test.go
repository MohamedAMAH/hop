package folder

import (
	"errors"
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
