package fake

import (
	"errors"
	"testing"

	"hop/internal/bundle"
	"hop/internal/transport"
)

func TestFakeSendReceive(t *testing.T) {
	f := New()
	if _, err := f.Receive("hop"); !errors.Is(err, transport.ErrNoBundle) {
		t.Fatalf("empty Receive must return ErrNoBundle, got %v", err)
	}
	b := &bundle.Bundle{Meta: bundle.Meta{ProjectID: "hop", Token: bundle.DefaultToken}}
	if err := f.Send(b); err != nil {
		t.Fatal(err)
	}
	got, err := f.Receive("hop")
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.ProjectID != "hop" {
		t.Fatalf("Receive returned wrong bundle: %+v", got.Meta)
	}
}
