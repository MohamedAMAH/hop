package claude

import (
	"os"
	"path/filepath"
	"strings"

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
	return os.Rename(tmpName, final)
}

var _ agent.Agent = Claude{}
