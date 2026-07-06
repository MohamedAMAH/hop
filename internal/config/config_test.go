package config

import (
	"path/filepath"
	"testing"
)

func TestPathFor(t *testing.T) {
	c := Config{
		Machine: "win",
		Projects: map[string]Project{
			"hop": {Paths: map[string]string{"win": `D:\hop`, "nix": "/home/x/hop"}},
		},
	}
	if p, ok := c.PathFor("hop", "nix"); !ok || p != "/home/x/hop" {
		t.Fatalf("PathFor = %q,%v", p, ok)
	}
	if _, ok := c.PathFor("hop", "mac"); ok {
		t.Fatal("unknown machine must return ok=false")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := Config{Machine: "win", Projects: map[string]Project{
		"hop": {Paths: map[string]string{"win": `D:\hop`}, Transport: "folder",
			TransportConfig: map[string]string{"dir": `E:\sync`}, Handoff: "manual"}}}
	if err := c.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Machine != "win" || got.Projects["hop"].Transport != "folder" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
