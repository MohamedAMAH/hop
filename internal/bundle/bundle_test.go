package bundle

import (
	"testing"

	"hop/internal/agent"
)

func TestSelectTokenDefaultWhenNoCollision(t *testing.T) {
	s := []agent.Session{{ID: "a", Data: []byte(`{"cwd":"/x"}`)}}
	if got := SelectToken(s); got != DefaultToken {
		t.Fatalf("SelectToken = %q, want %q", got, DefaultToken)
	}
}

func TestSelectTokenAvoidsCollision(t *testing.T) {
	s := []agent.Session{{ID: "a", Data: []byte(`{"msg":"literal __HOP_ROOT__ here"}`)}}
	got := SelectToken(s)
	if got == DefaultToken {
		t.Fatalf("SelectToken must avoid a colliding token, got default")
	}
	// The chosen alternate must not itself collide.
	for _, sess := range s {
		if contains(sess.Data, []byte(got)) {
			t.Fatalf("alternate token %q still collides", got)
		}
	}
}
