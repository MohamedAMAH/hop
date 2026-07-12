package lan

import "testing"

func TestManualPeer(t *testing.T) {
	d := ManualPeer("nixos", "10.0.0.9:8888")
	if d.Name != "nixos" || d.Address != "10.0.0.9:8888" || d.Fingerprint != "" {
		t.Fatalf("manual peer = %+v", d)
	}
}
