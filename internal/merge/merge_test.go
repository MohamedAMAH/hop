package merge

import "testing"

func TestDecide(t *testing.T) {
	cases := []struct {
		name            string
		local, incoming []byte
		want            Decision
	}{
		{"incoming only", nil, []byte("a\n"), New},
		{"local only", []byte("a\n"), nil, KeepLocalOnly},
		{"identical", []byte("a\nb\n"), []byte("a\nb\n"), NoOp},
		{"local prefix of incoming", []byte("a\n"), []byte("a\nb\n"), Update},
		{"incoming prefix of local", []byte("a\nb\n"), []byte("a\n"), KeepLocalNewer},
		{"diverged", []byte("a\nb\n"), []byte("a\nc\n"), Diverged},
	}
	for _, c := range cases {
		if got := Decide(c.local, c.incoming); got != c.want {
			t.Errorf("%s: Decide = %v, want %v", c.name, got, c.want)
		}
	}
}
