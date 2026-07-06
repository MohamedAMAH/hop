/* Package agent defines the interface hop uses to talk to a specific coding agent's on-disk session store. claude-code is the first implementation. */
package agent

/* Session is one agent session: its stable ID (filename without extension) and its raw transcript bytes (append-only JSONL for claude-code). */
type Session struct {
	ID   string
	Data []byte
}

/* Agent abstracts where and how an agent stores sessions for a project. home is the user's home directory; projectRoot is the project's absolute path on THIS machine. */
type Agent interface {
	ProjectDir(home, projectRoot string) string
	ListSessions(home, projectRoot string) ([]Session, error)
	WriteSession(home, projectRoot string, s Session) error
	/* BackupSession copies the current on-disk session to a sibling backup and
	returns the backup path, or an empty string when the session does not exist. */
	BackupSession(home, projectRoot, id string) (string, error)
}
