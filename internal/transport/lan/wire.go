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

/* wireBundle is the JSON envelope transferred between machines. */
type wireBundle struct {
	Meta     bundle.Meta   `json:"meta"`
	Sessions []wireSession `json:"sessions"`
}

/* hashData returns the lowercase hex SHA-256 of session bytes. */
func hashData(d []byte) string {
	sum := sha256.Sum256(d)
	return hex.EncodeToString(sum[:])
}

/* encodeBundle serializes a bundle to the wire envelope with per-session hashes. */
func encodeBundle(b *bundle.Bundle) ([]byte, error) {
	w := wireBundle{Meta: b.Meta}
	for _, s := range b.Sessions {
		w.Sessions = append(w.Sessions, wireSession{ID: s.ID, Hash: hashData(s.Data), Data: s.Data})
	}
	return json.Marshal(w)
}

/* decodeBundle parses the wire envelope and rejects any session whose hash does not match. */
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
	return b, nil
}
