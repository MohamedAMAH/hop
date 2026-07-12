package lan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

/* Peer is a machine this one has paired with over the LAN. */
type Peer struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	LastAddress string `json:"lastAddress"`
}

/* Peers is the set of paired machines, keyed by fingerprint. */
type Peers struct {
	mu   sync.RWMutex
	byFP map[string]Peer
}

/* LoadPeers reads the peer store; a missing file yields an empty set. */
func LoadPeers(path string) (*Peers, error) {
	p := &Peers{byFP: map[string]Peer{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Peer
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, peer := range list {
		p.byFP[peer.Fingerprint] = peer
	}
	return p, nil
}

/* Save writes the peer store atomically. */
func (p *Peers) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p.All(), "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "peers.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

/* ByFingerprint returns the peer with the given fingerprint, if present. */
func (p *Peers) ByFingerprint(fp string) (Peer, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	peer, ok := p.byFP[fp]
	return peer, ok
}

/* All returns the peers sorted by fingerprint for stable output. */
func (p *Peers) All() []Peer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Peer, 0, len(p.byFP))
	for _, peer := range p.byFP {
		out = append(out, peer)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint < out[j].Fingerprint })
	return out
}

/* Upsert adds or replaces a peer by fingerprint. */
func (p *Peers) Upsert(peer Peer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byFP[peer.Fingerprint] = peer
}
