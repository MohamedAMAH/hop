/*
Package bundle defines hop's machine-neutral session bundle and its metadata.
*/
package bundle

import (
	"bytes"
	"fmt"

	"hop/internal/agent"
)

/*
DefaultToken is the ASCII placeholder that replaces the project root in a
neutralized transcript.
*/
const DefaultToken = "__HOP_ROOT__"

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
	ProjectID string `json:"projectId"`
	Token     string `json:"token"`
	Baton     Baton  `json:"baton"`
}

/*
Bundle is a machine-neutral snapshot of all of a project's sessions.
*/
type Bundle struct {
	Meta     Meta
	Sessions []agent.Session
}

/*
SelectToken returns DefaultToken, or a non-colliding alternate if the
default literally occurs in any session's bytes.
*/
func SelectToken(sessions []agent.Session) string {
	candidate := DefaultToken
	for n := 0; ; n++ {
		if n > 0 {
			candidate = fmt.Sprintf("__HOP_ROOT_%d__", n)
		}
		if !anyContains(sessions, []byte(candidate)) {
			return candidate
		}
	}
}

func anyContains(sessions []agent.Session, tok []byte) bool {
	for _, s := range sessions {
		if contains(s.Data, tok) {
			return true
		}
	}
	return false
}

func contains(data, tok []byte) bool { return bytes.Contains(data, tok) }
