package lan

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"hop/internal/bundle"
	"hop/internal/transport"
)

/* Transport syncs with one paired peer over pinned mutual TLS. */
type Transport struct {
	id   Identity
	peer Peer
}

/* NewTransport returns a LAN transport for a paired peer. */
func NewTransport(id Identity, peer Peer) *Transport { return &Transport{id: id, peer: peer} }

/*
pinnedConfig returns a tls.Config that presents this identity and refuses any
server certificate whose fingerprint is not the expected one. InsecureSkipVerify
disables CA checking only because VerifyPeerCertificate enforces the pin.
*/
func pinnedConfig(id Identity, expectedFP string) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{id.Cert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 || fingerprintOf(rawCerts[0]) != expectedFP {
				return fmt.Errorf("lan: peer identity changed — refusing")
			}
			return nil
		},
	}
}

/* httpClient returns an HTTPS client pinned to the given fingerprint. */
func httpClient(id Identity, expectedFP string) *http.Client {
	return &http.Client{Timeout: 30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: pinnedConfig(id, expectedFP)}}
}

/* Send delivers a bundle to the peer, which materializes it. A 409 signals a peer conflict. */
func (t *Transport) Send(b *bundle.Bundle) error {
	data, err := encodeBundle(b)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://%s/sync/push?project=%s", t.peer.LastAddress, url.QueryEscape(b.Meta.ProjectID))
	resp, err := httpClient(t.id, t.peer.Fingerprint).Post(u, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<12))
		return fmt.Errorf("lan: %s could not accept the push (%s) — pull from it or resolve there first",
			t.peer.Name, bytes.TrimSpace(msg))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lan: push to %s failed: %s", t.peer.Name, resp.Status)
	}
	return nil
}

/* Receive asks the peer to capture its sessions and returns the bundle. */
func (t *Transport) Receive(projectID string) (*bundle.Bundle, error) {
	u := fmt.Sprintf("https://%s/sync/pull?project=%s", t.peer.LastAddress, url.QueryEscape(projectID))
	resp, err := httpClient(t.id, t.peer.Fingerprint).Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lan: pull from %s failed: %s", t.peer.Name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<30))
	if err != nil {
		return nil, err
	}
	return decodeBundle(data)
}

/* PairResult is the outcome of a pairing handshake awaiting user confirmation. */
type PairResult struct {
	PeerFingerprint string
	PeerName        string
	PeerAddress     string
	Code            string
}

/*
Pair performs the pairing handshake with a peer: it exchanges hellos over TLS,
learns the peer's certificate fingerprint, and returns the confirmation code the
user must verify on both machines. No fingerprint is pinned yet, so this dials
with a capture-only verifier that accepts the first certificate and records it.
*/
func Pair(id Identity, myName, myListenAddr, peerAddr string) (PairResult, error) {
	var peerFP string
	cfg := &tls.Config{
		Certificates:       []tls.Certificate{id.Cert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("lan: peer presented no certificate")
			}
			peerFP = fingerprintOf(rawCerts[0])
			return nil
		},
	}
	client := &http.Client{Timeout: 20 * time.Second,
		Transport: &http.Transport{TLSClientConfig: cfg}}
	body, _ := json.Marshal(hello{Name: myName, ListenAddress: myListenAddr})
	resp, err := client.Post("https://"+peerAddr+"/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		return PairResult{}, err
	}
	defer resp.Body.Close()
	var peerHello hello
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&peerHello); err != nil {
		return PairResult{}, err
	}
	return PairResult{
		PeerFingerprint: peerFP,
		PeerName:        peerHello.Name,
		PeerAddress:     peerHello.ListenAddress,
		Code:            Code(id.Fingerprint, peerFP),
	}, nil
}

var _ transport.Transport = (*Transport)(nil)
