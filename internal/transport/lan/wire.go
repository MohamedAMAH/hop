package lan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"hop/internal/agent"
	"hop/internal/bundle"
)

/* wireSession is one session on the wire; Data is base64-encoded by encoding/json. */
type wireSession struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
	Data []byte `json:"data"`
}

/* wireFile is one non-transcript file on the wire, with its bytes hashed for integrity. */
type wireFile struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Data    []byte `json:"data"`
	ModTime int64  `json:"modTime"`
}

/* wireBundle is the JSON envelope transferred between machines. */
type wireBundle struct {
	Meta     bundle.Meta   `json:"meta"`
	Sessions []wireSession `json:"sessions"`
	Files    []wireFile    `json:"files"`
}

/* hashData returns the lowercase hex SHA-256 of session bytes. */
func hashData(d []byte) string {
	sum := sha256.Sum256(d)
	return hex.EncodeToString(sum[:])
}

/* encodeBundle serializes a bundle to the wire envelope with per-session and per-file hashes. */
func encodeBundle(b *bundle.Bundle) ([]byte, error) {
	w := wireBundle{Meta: b.Meta}
	for _, s := range b.Sessions {
		w.Sessions = append(w.Sessions, wireSession{ID: s.ID, Hash: hashData(s.Data), Data: s.Data})
	}
	for _, f := range b.Files {
		w.Files = append(w.Files, wireFile{Path: f.Path, Hash: f.Hash, Data: f.Data, ModTime: f.ModTime})
	}
	return json.Marshal(w)
}

/* decodeBundle parses the wire envelope and rejects any session or file whose hash does not match. */
func decodeBundle(data []byte) (*bundle.Bundle, error) {
	var w wireBundle
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	b := &bundle.Bundle{Meta: w.Meta}
	for _, s := range w.Sessions {
		if hashData(s.Data) != s.Hash {
			return nil, fmt.Errorf("lan: session %s failed its integrity hash", s.ID)
		}
		b.Sessions = append(b.Sessions, agent.Session{ID: s.ID, Data: s.Data})
	}
	for _, f := range w.Files {
		if err := bundle.ValidFilePath(f.Path); err != nil {
			return nil, fmt.Errorf("lan: file %s has an invalid path: %w", f.Path, err)
		}
		if bundle.HashBytes(f.Data) != f.Hash {
			return nil, fmt.Errorf("lan: file %s failed its integrity hash", f.Path)
		}
		b.Files = append(b.Files, bundle.FileEntry{Path: f.Path, Data: f.Data, Hash: f.Hash, ModTime: f.ModTime})
	}
	return b, nil
}
