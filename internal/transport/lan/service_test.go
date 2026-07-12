package lan

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hop/internal/agent"
	"hop/internal/agent/claude"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/syncer"
	"hop/internal/transport/fake"
)

/* testDeps builds a DepsFunc rooted at a fresh temp home for a single project. */
func testDeps(t *testing.T, machine, projectID, root string) (DepsFunc, string) {
	t.Helper()
	home := t.TempDir()
	cfg := config.Config{Machine: machine, Projects: map[string]config.Project{
		projectID: {Paths: map[string]string{machine: root}}}}
	deps := syncer.Deps{Cfg: cfg, Agent: claude.New(), Home: home, StateDir: t.TempDir(), OS: osinfo.Unix}
	return func(id string) (syncer.Deps, error) { return deps, nil }, home
}

/* tlsClient returns an http.Client presenting id's certificate and skipping server verification. */
func tlsClient(id Identity) *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		Certificates:       []tls.Certificate{id.Cert},
		InsecureSkipVerify: true,
	}}}
}

func TestServicePairRecordsPending(t *testing.T) {
	idServer, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	idClient, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	peers := &Peers{byFP: map[string]Peer{}}
	svc := NewService(idServer, peers, "", "server", "server:9999", nil)
	srv := httptest.NewUnstartedServer(svc.Handler())
	srv.TLS = svc.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	body := bytes.NewBufferString(`{"name":"client","listenAddress":"client:8888"}`)
	resp, err := tlsClient(idClient).Post(srv.URL+"/pair", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	pend := svc.Pending()
	if len(pend) != 1 || pend[0].Name != "client" || pend[0].Fingerprint != idClient.Fingerprint {
		t.Fatalf("pending = %+v", pend)
	}
	if pend[0].Code != Code(idServer.Fingerprint, idClient.Fingerprint) {
		t.Fatalf("code mismatch: %s", pend[0].Code)
	}

	// Confirm pins the peer into the store.
	if err := svc.Confirm(idClient.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, ok := peers.ByFingerprint(idClient.Fingerprint); !ok {
		t.Fatal("Confirm did not pin the peer")
	}
	if len(svc.Pending()) != 0 {
		t.Fatal("Confirm did not clear the pending entry")
	}
}

func TestServiceRejectsUnpinnedSync(t *testing.T) {
	idServer, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	idClient, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	peers := &Peers{byFP: map[string]Peer{}} // client NOT pinned.
	depsFor, _ := testDeps(t, "server", "demo", "/proj")
	svc := NewService(idServer, peers, "", "server", "", depsFor)
	srv := httptest.NewUnstartedServer(svc.Handler())
	srv.TLS = svc.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	resp, err := tlsClient(idClient).Get(srv.URL + "/sync/pull?project=demo")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unpinned client should be 403, got %d", resp.StatusCode)
	}
}

func TestServiceSyncPushMaterializesAndPullReturnsBundle(t *testing.T) {
	idServer, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	idClient, err := LoadOrCreateIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	peers := &Peers{byFP: map[string]Peer{
		idClient.Fingerprint: {Name: "client", Fingerprint: idClient.Fingerprint},
	}}
	serverRoot := "/home/server/proj"
	depsFor, serverHome := testDeps(t, "server", "demo", serverRoot)
	svc := NewService(idServer, peers, "", "server", "", depsFor)
	srv := httptest.NewUnstartedServer(svc.Handler())
	srv.TLS = svc.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	// Build a client-side bundle the same way a real peer would: capture and
	// neutralize a session through the sync engine into an in-memory seam.
	fixture, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "sample.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	clientRoot := "/src/proj"
	clientHome := t.TempDir()
	if err := claude.New().WriteSession(clientHome, clientRoot, agent.Session{ID: "5038b5e4", Data: fixture}); err != nil {
		t.Fatal(err)
	}
	clientSeam := fake.New()
	clientDeps := syncer.Deps{
		Cfg:       config.Config{Machine: "client", Projects: map[string]config.Project{"demo": {Paths: map[string]string{"client": clientRoot}}}},
		Agent:     claude.New(),
		Transport: clientSeam,
		Home:      clientHome,
		StateDir:  t.TempDir(),
		OS:        osinfo.Unix,
	}
	if _, err := syncer.Push(clientDeps, "demo", "2026-07-06T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	b, err := clientSeam.Receive("demo")
	if err != nil {
		t.Fatal(err)
	}
	wireData, err := encodeBundle(b)
	if err != nil {
		t.Fatal(err)
	}

	// /sync/push: the pinned client sends the bundle for the server to materialize.
	resp, err := tlsClient(idClient).Post(srv.URL+"/sync/push?project=demo", "application/json", bytes.NewReader(wireData))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push: expected 200, got %d", resp.StatusCode)
	}

	got, err := claude.New().ListSessions(serverHome, serverRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 materialized session on the server, got %d", len(got))
	}
	text := string(got[0].Data)
	if strings.Contains(text, clientRoot) {
		t.Fatalf("client root leaked into materialized session:\n%s", text)
	}
	if !strings.Contains(text, serverRoot) {
		t.Fatalf("cwd not rewritten to server root:\n%s", text)
	}

	// /sync/pull: the pinned client asks the server to capture and return its sessions.
	resp, err = tlsClient(idClient).Get(srv.URL + "/sync/pull?project=demo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pull: expected 200, got %d", resp.StatusCode)
	}
	var raw bytes.Buffer
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	pulled, err := decodeBundle(raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if pulled.Meta.ProjectID != "demo" || len(pulled.Sessions) != 1 {
		t.Fatalf("pulled bundle = %+v", pulled.Meta)
	}
	// Sanity: the wire body must actually be JSON (no stray content-type surprise).
	var probe json.RawMessage
	if err := json.Unmarshal(raw.Bytes(), &probe); err != nil {
		t.Fatalf("pulled bundle is not valid JSON: %v", err)
	}
}
