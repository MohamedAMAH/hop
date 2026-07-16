/*
Package folder is a Transport backed by a local directory. Whatever keeps
that directory synced across machines (Syncthing, Dropbox, a network share)
is entirely external; this package only reads and writes files.
*/
package folder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

/*
manifestEntry records the identity and expected contents of one session
file, so Receive can detect a partial or stale bundle before trusting it.
*/
type manifestEntry struct {
	ID    string `json:"id"`
	Hash  string `json:"hash"`
	Bytes int64  `json:"bytes"`
}

/* fileEntry records one synced non-transcript file in the manifest. */
type fileEntry struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	ModTime int64  `json:"modTime"`
}

/*
diskMeta is the on-disk envelope written to meta.json. It carries the
domain bundle.Meta plus a manifest of the session files and a manifest of
the non-transcript files that belong to it; neither manifest leaks into
bundle.Meta itself.
*/
type diskMeta struct {
	Meta     bundle.Meta     `json:"meta"`
	Manifest []manifestEntry `json:"manifest"`
	Files    []fileEntry     `json:"files"`
}

func (f *Folder) projectDir(projectID string) string {
	return filepath.Join(f.dir, projectID)
}

/*
Send writes one <id>.jsonl per session, prunes stale session files left
over from an older push, writes the bundle's files under files/, then
writes meta.json last as the commit marker.
*/
func (f *Folder) Send(b *bundle.Bundle) error {
	pd := f.projectDir(b.Meta.ProjectID)
	if err := os.MkdirAll(pd, 0o755); err != nil {
		return err
	}
	manifest := make([]manifestEntry, 0, len(b.Sessions))
	kept := make(map[string]bool, len(b.Sessions))
	for _, s := range b.Sessions {
		if err := writeAtomic(filepath.Join(pd, s.ID+".jsonl"), s.Data); err != nil {
			return err
		}
		sum := sha256.Sum256(s.Data)
		manifest = append(manifest, manifestEntry{
			ID:    s.ID,
			Hash:  hex.EncodeToString(sum[:]),
			Bytes: int64(len(s.Data)),
		})
		kept[s.ID] = true
	}
	// Best-effort prune of session files left over from an older push; Receive's manifest check is the real guarantee.
	pruneStaleSessionFiles(pd, kept)
	fileManifest := make([]fileEntry, 0, len(b.Files))
	for _, fe := range b.Files {
		if err := bundle.ValidFilePath(fe.Path); err != nil {
			return err
		}
		dest := filepath.Join(pd, "files", filepath.FromSlash(fe.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeAtomic(dest, fe.Data); err != nil {
			return err
		}
		fileManifest = append(fileManifest, fileEntry{Path: fe.Path, Hash: fe.Hash, ModTime: fe.ModTime})
	}
	dm := diskMeta{Meta: b.Meta, Manifest: manifest, Files: fileManifest}
	metaBytes, err := json.MarshalIndent(dm, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(pd, "meta.json"), metaBytes)
}

/* pruneStaleSessionFiles removes *.jsonl files in dir whose ID is not in kept, ignoring any remove errors. */
func pruneStaleSessionFiles(dir string, kept map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		if kept[id] {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
}

/*
Receive reads the bundle for projectID, or transport.ErrNoBundle if absent.
Every session manifest entry and every files manifest entry is validated
against the file on disk; a missing file or a byte-length/hash mismatch
returns transport.ErrIncompleteBundle rather than a partial bundle.
*/
func (f *Folder) Receive(projectID string) (*bundle.Bundle, error) {
	pd := f.projectDir(projectID)
	metaBytes, err := os.ReadFile(filepath.Join(pd, "meta.json"))
	if os.IsNotExist(err) {
		return nil, transport.ErrNoBundle
	}
	if err != nil {
		return nil, err
	}
	var dm diskMeta
	if err := json.Unmarshal(metaBytes, &dm); err != nil {
		return nil, err
	}
	sessions := make([]agent.Session, 0, len(dm.Manifest))
	for _, entry := range dm.Manifest {
		data, err := os.ReadFile(filepath.Join(pd, entry.ID+".jsonl"))
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: missing session %s", transport.ErrIncompleteBundle, entry.ID)
		}
		if err != nil {
			return nil, err
		}
		if int64(len(data)) != entry.Bytes {
			return nil, fmt.Errorf("%w: session %s length mismatch", transport.ErrIncompleteBundle, entry.ID)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != entry.Hash {
			return nil, fmt.Errorf("%w: session %s hash mismatch", transport.ErrIncompleteBundle, entry.ID)
		}
		sessions = append(sessions, agent.Session{ID: entry.ID, Data: data})
	}
	files := make([]bundle.FileEntry, 0, len(dm.Files))
	for _, fe := range dm.Files {
		if err := bundle.ValidFilePath(fe.Path); err != nil {
			return nil, fmt.Errorf("hop: bundle file manifest has an invalid path: %w", err)
		}
		data, err := os.ReadFile(filepath.Join(pd, "files", filepath.FromSlash(fe.Path)))
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: missing file %s", transport.ErrIncompleteBundle, fe.Path)
		}
		if err != nil {
			return nil, err
		}
		if bundle.HashBytes(data) != fe.Hash {
			return nil, fmt.Errorf("%w: file %s failed its integrity check", transport.ErrIncompleteBundle, fe.Path)
		}
		files = append(files, bundle.FileEntry{Path: fe.Path, Data: data, Hash: fe.Hash, ModTime: fe.ModTime})
	}
	return &bundle.Bundle{Meta: dm.Meta, Sessions: sessions, Files: files}, nil
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
