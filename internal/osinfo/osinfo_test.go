package osinfo

import "testing"

func TestSep(t *testing.T) {
	if got := Unix.Sep(); got != "/" {
		t.Fatalf("Unix.Sep() = %q, want %q", got, "/")
	}
	if got := Windows.Sep(); got != `\` {
		t.Fatalf("Windows.Sep() = %q, want %q", got, `\`)
	}
}

func TestCurrentIsKnown(t *testing.T) {
	switch Current() {
	case Unix, Windows:
	default:
		t.Fatalf("Current() returned unknown OS %d", Current())
	}
}
