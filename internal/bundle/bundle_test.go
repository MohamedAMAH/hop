package bundle

import (
	"strings"
	"testing"

	"hop/internal/agent"
)

func TestSelectTokensAreDistinctAndNonColliding(t *testing.T) {
	sessions := []agent.Session{{ID: "s", Data: []byte(`{"x":"__HOP_ROOT__ and __HOP_STORE__"}`)}}
	files := []FileEntry{{Path: "memory/m.md", Data: []byte("__HOP_ROOT_1__")}}
	root, prefix := SelectTokens(sessions, files)
	if root == prefix {
		t.Fatalf("tokens must differ: %q", root)
	}
	all := string(sessions[0].Data) + string(files[0].Data)
	if strings.Contains(all, root) || strings.Contains(all, prefix) {
		t.Fatalf("tokens collide with content: root=%q prefix=%q", root, prefix)
	}
}

func TestHashBytesIsStable(t *testing.T) {
	if HashBytes([]byte("abc")) != HashBytes([]byte("abc")) {
		t.Fatal("hash not stable")
	}
	if HashBytes([]byte("abc")) == HashBytes([]byte("abd")) {
		t.Fatal("hash collision on different input")
	}
}
