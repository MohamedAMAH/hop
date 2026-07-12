package lan

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"hop/internal/transport"
)

func TestTransportRefusesWrongFingerprint(t *testing.T) {
	idServer, _ := LoadOrCreateIdentity(t.TempDir())
	other, _ := LoadOrCreateIdentity(t.TempDir()) // a different machine's fingerprint.
	idClient, _ := LoadOrCreateIdentity(t.TempDir())
	peers := &Peers{byFP: map[string]Peer{idClient.Fingerprint: {Fingerprint: idClient.Fingerprint}}}
	svc := NewService(idServer, peers, "", "server", "", nil)
	srv := httptest.NewUnstartedServer(svc.Handler())
	srv.TLS = svc.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "https://")
	// Pin the WRONG fingerprint (other), so the client must refuse the real server.
	tr := NewTransport(idClient, Peer{Fingerprint: other.Fingerprint, LastAddress: addr})
	_, err := tr.Receive("demo")
	if err == nil {
		t.Fatal("client must refuse a server whose fingerprint is not the pinned one")
	}
	var _ transport.Transport = tr
}

func TestPairComputesMatchingCode(t *testing.T) {
	idServer, _ := LoadOrCreateIdentity(t.TempDir())
	idClient, _ := LoadOrCreateIdentity(t.TempDir())
	peers := &Peers{byFP: map[string]Peer{}}
	svc := NewService(idServer, peers, "", "server", "server:9", nil)
	srv := httptest.NewUnstartedServer(svc.Handler())
	srv.TLS = svc.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "https://")
	res, err := Pair(idClient, "client", "client:8", addr)
	if err != nil {
		t.Fatal(err)
	}
	if res.PeerFingerprint != idServer.Fingerprint || res.PeerName != "server" {
		t.Fatalf("pair result = %+v", res)
	}
	if res.Code != Code(idClient.Fingerprint, idServer.Fingerprint) {
		t.Fatalf("code mismatch: %s", res.Code)
	}
	_ = errors.New
}
