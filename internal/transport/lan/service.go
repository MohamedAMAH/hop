package lan

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"hop/internal/syncer"
	"hop/internal/transport/fake"
)

/* DepsFunc builds the local syncer.Deps for a project; Transport is set by the service. */
type DepsFunc func(projectID string) (syncer.Deps, error)

/* PendingPair is an unconfirmed pairing request awaiting the user's confirmation. */
type PendingPair struct {
	Fingerprint string
	Name        string
	Address     string
	Code        string
}

/* hello is the small identity exchange used during pairing. */
type hello struct {
	Name          string `json:"name"`
	ListenAddress string `json:"listenAddress"`
}

/* Service is the LAN peer service handling pairing and sync over mutual TLS. */
type Service struct {
	id            Identity
	peers         *Peers
	peersPath     string
	name          string
	advertiseAddr string
	depsFor       DepsFunc
	pending       map[string]PendingPair
}

/* NewService returns a peer service for this machine's identity and paired peers. */
func NewService(id Identity, peers *Peers, peersPath, machineName, advertiseAddr string, depsFor DepsFunc) *Service {
	return &Service{id: id, peers: peers, peersPath: peersPath, name: machineName,
		advertiseAddr: advertiseAddr, depsFor: depsFor, pending: map[string]PendingPair{}}
}

/* Handler returns the HTTP routes for pairing and sync. */
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/pair", s.handlePair)
	mux.HandleFunc("/sync/push", s.requirePinned(s.handlePush))
	mux.HandleFunc("/sync/pull", s.requirePinned(s.handlePull))
	return mux
}

/* clientFingerprint returns the fingerprint of the presented client certificate, if any. */
func clientFingerprint(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	return fingerprintOf(r.TLS.PeerCertificates[0].Raw)
}

/* handlePair records the caller as a pending pair and returns this machine's hello. */
func (s *Service) handlePair(w http.ResponseWriter, r *http.Request) {
	fp := clientFingerprint(r)
	if fp == "" {
		http.Error(w, "client certificate required", http.StatusBadRequest)
		return
	}
	var h hello
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&h); err != nil {
		http.Error(w, "bad hello", http.StatusBadRequest)
		return
	}
	s.pending[fp] = PendingPair{Fingerprint: fp, Name: h.Name, Address: h.ListenAddress,
		Code: Code(s.id.Fingerprint, fp)}
	json.NewEncoder(w).Encode(hello{Name: s.name, ListenAddress: s.advertiseAddr})
}

/* requirePinned wraps a handler so only a pinned client certificate may proceed. */
func (s *Service) requirePinned(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fp := clientFingerprint(r)
		if fp == "" {
			http.Error(w, "client certificate required", http.StatusForbidden)
			return
		}
		if _, ok := s.peers.ByFingerprint(fp); !ok {
			http.Error(w, "not a paired machine", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

/* handlePush materializes a received bundle locally, aborting on a peer conflict. */
func (s *Service) handlePush(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<30))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, err := decodeBundle(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := s.depsFor(project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	seam := fake.New()
	if err := seam.Send(b); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.Transport = seam
	if _, err := syncer.Pull(d, project, now(), syncer.AbortResolver{}); err != nil {
		if errors.Is(err, syncer.ErrDiverged) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

/* handlePull captures and neutralizes this machine's sessions and returns the bundle. */
func (s *Service) handlePull(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	d, err := s.depsFor(project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	seam := fake.New()
	d.Transport = seam
	if _, err := syncer.Push(d, project, now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := seam.Receive(project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out, err := encodeBundle(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

/* Pending returns the pairing requests awaiting confirmation. */
func (s *Service) Pending() []PendingPair {
	out := make([]PendingPair, 0, len(s.pending))
	for _, p := range s.pending {
		out = append(out, p)
	}
	return out
}

/* Confirm pins a pending peer and persists the peer store. */
func (s *Service) Confirm(fp string) error {
	p, ok := s.pending[fp]
	if !ok {
		return errors.New("no such pending pair")
	}
	s.peers.Upsert(Peer{Name: p.Name, Fingerprint: p.Fingerprint, LastAddress: p.Address})
	delete(s.pending, fp)
	if s.peersPath == "" {
		return nil
	}
	return s.peers.Save(s.peersPath)
}

/* Reject discards a pending pairing request. */
func (s *Service) Reject(fp string) { delete(s.pending, fp) }

/* now returns the current time as an RFC3339 string. */
func now() string { return time.Now().UTC().Format(time.RFC3339) }

/* ServerTLSConfig returns a tls.Config presenting this identity and requesting a client cert. */
func (s *Service) ServerTLSConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{s.id.Cert}, ClientAuth: tls.RequireAnyClientCert}
}
