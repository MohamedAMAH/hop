/*
Package bundle defines hop's machine-neutral session bundle and its metadata.
*/
package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"hop/internal/agent"
)

/*
DefaultToken is the ASCII placeholder that replaces the project root in a
neutralized transcript.
*/
const DefaultToken = "__HOP_ROOT__"

/*
DefaultPrefixToken is the ASCII placeholder that replaces the claude
project-storage prefix (~/.claude/projects/<encoded>/) in a neutralized
transcript.
*/
const DefaultPrefixToken = "__HOP_STORE__"

/*
Baton is the one-active-at-a-time ownership marker that travels in a
bundle.
*/
type Baton struct {
	Owner     string `json:"owner"`
	Sequence  int    `json:"sequence"`
	UpdatedAt string `json:"updatedAt"`
}

/*
Meta is the bundle's non-session metadata.
*/
type Meta struct {
	ProjectID   string `json:"projectId"`
	Token       string `json:"token"`
	PrefixToken string `json:"prefixToken"`
	Baton       Baton  `json:"baton"`
}

/*
Bundle is a machine-neutral snapshot of all of a project's sessions.
*/
type Bundle struct {
	Meta     Meta
	Sessions []agent.Session
	Files    []FileEntry
}

/*
FileEntry is one non-transcript project file carried in a bundle: its
project-storage-relative path (always '/'-separated), its bytes, a SHA-256
hex hash of those bytes, and its modification time in Unix nanoseconds.
*/
type FileEntry struct {
	Path    string `json:"path"`
	Data    []byte `json:"data"`
	Hash    string `json:"hash"`
	ModTime int64  `json:"modTime"`
}

/* HashBytes returns the hex SHA-256 of data. */
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

/*
SelectTokens returns two distinct placeholders — one for the project root, one
for the storage prefix — neither of which occurs in any session or file bytes.
*/
func SelectTokens(sessions []agent.Session, files []FileEntry) (root, prefix string) {
	root = selectToken(DefaultToken, sessions, files, "")
	prefix = selectToken(DefaultPrefixToken, sessions, files, root)
	return root, prefix
}

/* selectToken returns base, or a numbered variant, that occurs nowhere in the content and differs from avoid. */
func selectToken(base string, sessions []agent.Session, files []FileEntry, avoid string) string {
	candidate := base
	for n := 0; ; n++ {
		if n > 0 {
			candidate = fmt.Sprintf("%s_%d__", strings.TrimSuffix(base, "__"), n)
		}
		if candidate != avoid && !contentContains(sessions, files, []byte(candidate)) {
			return candidate
		}
	}
}

/* contentContains reports whether tok occurs in any session or file bytes. */
func contentContains(sessions []agent.Session, files []FileEntry, tok []byte) bool {
	for _, s := range sessions {
		if bytes.Contains(s.Data, tok) {
			return true
		}
	}
	for _, f := range files {
		if bytes.Contains(f.Data, tok) {
			return true
		}
	}
	return false
}
