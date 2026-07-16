/* Package agent defines the interface hop uses to talk to a specific coding agent's on-disk session store. claude-code is the first implementation. */
package agent

/* Session is one agent session: its stable ID (filename without extension) and its raw transcript bytes (append-only JSONL for claude-code). */
type Session struct {
	ID   string
	Data []byte
}

/* Kind classifies a non-transcript project file by how it merges across machines. */
type Kind int

const (
	KindSidecar Kind = iota // Write-once session sidecar (subagents, tool-results).
	KindMemory              // Editable project memory, merged newest-wins.
)

/* Artifact is one non-transcript project file: its '/'-separated path relative to the project storage dir, its bytes, and its modification time in Unix nanoseconds. */
type Artifact struct {
	RelPath string
	Data    []byte
	ModTime int64
}

/* Agent abstracts where and how an agent stores sessions for a project. home is the user's home directory; projectRoot is the project's absolute path on THIS machine. */
type Agent interface {
	ProjectDir(home, projectRoot string) string
	ListSessions(home, projectRoot string) ([]Session, error)
	WriteSession(home, projectRoot string, s Session) error
	/* BackupSession copies the current on-disk session to a sibling backup and
	returns the backup path, or an empty string when the session does not exist. */
	BackupSession(home, projectRoot, id string) (string, error)
	/* ListArtifacts returns every non-transcript file under the project store, excluding hop's own backups. */
	ListArtifacts(home, projectRoot string) ([]Artifact, error)
	/* WriteArtifact writes one artifact atomically, creating parent directories. */
	WriteArtifact(home, projectRoot string, a Artifact) error
	/* ReadArtifact returns an artifact's bytes and mod-time, reporting exists=false when it is absent. */
	ReadArtifact(home, projectRoot, relPath string) (data []byte, modTime int64, exists bool, err error)
	/* Classify reports how relPath merges across machines. */
	Classify(relPath string) Kind
}
