package lan

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreateIdentityIsStable(t *testing.T) {
	dir := t.TempDir()
	a, err := LoadOrCreateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	// A second load returns the SAME identity (persisted, not regenerated).
	b, err := LoadOrCreateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprint changed across loads: %s vs %s", a.Fingerprint, b.Fingerprint)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}
}
