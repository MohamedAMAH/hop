package claude

import (
	"fmt"
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

var _ agent.Agent = Claude{}
