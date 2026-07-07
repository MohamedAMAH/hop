package ui

import "testing"

func TestInteractiveHonorsPlain(t *testing.T) {
	orig := DetectTerminal
	defer func() { DetectTerminal = orig }()
	DetectTerminal = func() bool { return true }

	if Interactive(true) {
		t.Fatal("plain=true must force non-interactive")
	}
	if !Interactive(false) {
		t.Fatal("plain=false with a terminal must be interactive")
	}
	DetectTerminal = func() bool { return false }
	if Interactive(false) {
		t.Fatal("no terminal must be non-interactive")
	}
}
