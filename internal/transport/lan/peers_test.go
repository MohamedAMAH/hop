package lan

import (
	"path/filepath"
	"testing"
)

func TestPeersRoundTripAndUpsert(t *testing.T) {
	p, err := LoadPeers(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.All()) != 0 {
		t.Fatal("missing file should load empty")
	}
	p.Upsert(Peer{Name: "nixos", Fingerprint: "fp1", LastAddress: "10.0.0.5:1111"})
	p.Upsert(Peer{Name: "nixos", Fingerprint: "fp1", LastAddress: "10.0.0.9:2222"}) // same fp updates.
	if len(p.All()) != 1 {
		t.Fatalf("upsert on same fingerprint should not duplicate: %d", len(p.All()))
	}
	got, ok := p.ByFingerprint("fp1")
	if !ok || got.LastAddress != "10.0.0.9:2222" {
		t.Fatalf("ByFingerprint = %+v ok=%v", got, ok)
	}

	path := filepath.Join(t.TempDir(), "peers.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := LoadPeers(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.ByFingerprint("fp1"); !ok {
		t.Fatal("peer did not survive save/load")
	}
}
