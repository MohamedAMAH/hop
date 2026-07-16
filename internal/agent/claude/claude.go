package claude

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop/internal/agent"
)

/* Claude implements agent.Agent for claude-code's ~/.claude session store. */
type Claude struct{}

/* New returns a claude-code adapter. */
func New() Claude { return Claude{} }

/* ProjectDir returns ~/.claude/projects/<encoded-cwd> for projectRoot. */
func (Claude) ProjectDir(home, projectRoot string) string {
	return filepath.Join(home, ".claude", "projects", EncodeDir(projectRoot))
}

/* ListSessions reads every *.jsonl transcript in the project dir. A missing directory yields an empty slice, not an error. */
func (c Claude) ListSessions(home, projectRoot string) ([]agent.Session, error) {
	dir := c.ProjectDir(home, projectRoot)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []agent.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		// Read the transcript bytes for this session.
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		out = append(out, agent.Session{ID: id, Data: data})
	}
	return out, nil
}

/* WriteSession writes one session atomically (temp file + rename) into the project dir, creating it if needed. */
func (c Claude) WriteSession(home, projectRoot string, s agent.Session) error {
	dir := c.ProjectDir(home, projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, s.ID+".jsonl")
	tmp, err := os.CreateTemp(dir, s.ID+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(s.Data); err != nil {
		// Clean up the partially written temp file before returning.
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

/* BackupSession copies the on-disk session id into a timestamped file under .hop-backups and returns its path, or ("", nil) when the session does not exist; .hop-backups is a subdirectory so ListSessions never sees it. */
func (c Claude) BackupSession(home, projectRoot, id string) (string, error) {
	dir := c.ProjectDir(home, projectRoot)
	src := filepath.Join(dir, id+".jsonl")
	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	backupDir := filepath.Join(dir, ".hop-backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(backupDir, fmt.Sprintf("%s.%d.jsonl", id, time.Now().UnixNano()))
	tmp, err := os.CreateTemp(backupDir, id+".*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		// Clean up the partially written temp file before returning.
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	return final, nil
}

/* Classify reports memory files by their leading path segment and treats everything else as a write-once sidecar. */
func (Claude) Classify(relPath string) agent.Kind {
	if relPath == "memory" || strings.HasPrefix(relPath, "memory/") {
		return agent.KindMemory
	}
	return agent.KindSidecar
}

/*
ListArtifacts walks the project store and returns every file except the
top-level transcripts and hop's own .hop-backups. Each returned path is
relative and '/'-separated. A file whose size or mtime changes across the
read is skipped, since it is still being written.
*/
func (c Claude) ListArtifacts(home, projectRoot string) ([]agent.Artifact, error) {
	dir := c.ProjectDir(home, projectRoot)
	var out []agent.Artifact
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if e.IsDir() {
			if e.Name() == ".hop-backups" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// Top-level *.jsonl are transcripts handled by ListSessions.
		if !strings.Contains(rel, "/") && strings.HasSuffix(rel, ".jsonl") {
			return nil
		}
		art, ok, err := stableRead(p, rel)
		if err != nil {
			return err
		}
		if ok {
			out = append(out, art)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

/* stableRead reads p and returns its artifact only when size and mtime are unchanged across the read. */
func stableRead(p, rel string) (agent.Artifact, bool, error) {
	before, err := os.Stat(p)
	if err != nil {
		return agent.Artifact{}, false, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return agent.Artifact{}, false, err
	}
	after, err := os.Stat(p)
	if err != nil {
		return agent.Artifact{}, false, err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return agent.Artifact{}, false, nil
	}
	return agent.Artifact{RelPath: rel, Data: data, ModTime: after.ModTime().UnixNano()}, true, nil
}

/* WriteArtifact writes one artifact atomically (temp file + rename) under the project store, creating parent directories. */
func (c Claude) WriteArtifact(home, projectRoot string, a agent.Artifact) error {
	dir := c.ProjectDir(home, projectRoot)
	dest := filepath.Join(dir, filepath.FromSlash(a.RelPath))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), "hop.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(a.Data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return err
	}
	if a.ModTime != 0 {
		t := time.Unix(0, a.ModTime)
		if err := os.Chtimes(dest, t, t); err != nil {
			return fmt.Errorf("hop: set mod time on %s: %w", a.RelPath, err)
		}
	}
	return nil
}

/* ReadArtifact returns an artifact's bytes and mod-time, or exists=false when it is absent. */
func (c Claude) ReadArtifact(home, projectRoot, relPath string) ([]byte, int64, bool, error) {
	dest := filepath.Join(c.ProjectDir(home, projectRoot), filepath.FromSlash(relPath))
	info, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.ModTime().UnixNano(), true, nil
}

/* RewritesPaths reports that claude transcripts (.jsonl files) embed machine-specific paths, except memory files, which are preserved byte-for-byte. */
func (c Claude) RewritesPaths(relPath string) bool {
	return strings.HasSuffix(relPath, ".jsonl") && c.Classify(relPath) != agent.KindMemory
}

var _ agent.Agent = Claude{}
