/*
Package folder is a Transport backed by a local directory. Whatever keeps
that directory synced across machines (Syncthing, Dropbox, a network share)
is entirely external; this package only reads and writes files.
*/
package folder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"hop/internal/agent"
	"hop/internal/bundle"
	"hop/internal/transport"
)

/* Folder reads and writes bundles under a base directory. */
type Folder struct{ dir string }

/* New returns a Folder transport rooted at dir. */
func New(dir string) *Folder { return &Folder{dir: dir} }

func (f *Folder) projectDir(projectID string) string {
	return filepath.Join(f.dir, projectID)
}

/* Send writes meta.json plus one <id>.jsonl per session, each atomically. */
func (f *Folder) Send(b *bundle.Bundle) error {
	pd := f.projectDir(b.Meta.ProjectID)
	if err := os.MkdirAll(pd, 0o755); err != nil {
		return err
	}
	metaBytes, err := json.MarshalIndent(b.Meta, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(pd, "meta.json"), metaBytes); err != nil {
		return err
	}
	for _, s := range b.Sessions {
		if err := writeAtomic(filepath.Join(pd, s.ID+".jsonl"), s.Data); err != nil {
			return err
		}
	}
	return nil
}

/* Receive reads the bundle for projectID, or transport.ErrNoBundle if absent. */
func (f *Folder) Receive(projectID string) (*bundle.Bundle, error) {
	pd := f.projectDir(projectID)
	metaBytes, err := os.ReadFile(filepath.Join(pd, "meta.json"))
	if os.IsNotExist(err) {
		return nil, transport.ErrNoBundle
	}
	if err != nil {
		return nil, err
	}
	var meta bundle.Meta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(pd)
	if err != nil {
		return nil, err
	}
	var sessions []agent.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(pd, e.Name()))
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, agent.Session{
			ID:   strings.TrimSuffix(e.Name(), ".jsonl"),
			Data: data,
		})
	}
	return &bundle.Bundle{Meta: meta, Sessions: sessions}, nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hop.*.tmp")
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
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

var _ transport.Transport = (*Folder)(nil)
