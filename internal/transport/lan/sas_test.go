package lan

import (
	"regexp"
	"testing"
)

func TestCodeIsSixDigitsAndOrderIndependent(t *testing.T) {
	c := Code("aaaa", "bbbb")
	if !regexp.MustCompile(`^\d{6}$`).MatchString(c) {
		t.Fatalf("not a 6-digit code: %q", c)
	}
	// Both machines compute over the same pair regardless of who is local.
	if Code("aaaa", "bbbb") != Code("bbbb", "aaaa") {
		t.Fatal("code must be independent of argument order")
	}
}

func TestCodeDetectsMITM(t *testing.T) {
	// Honest: A and B both see each other. MITM: each sees M instead.
	honestA := Code("A", "B")
	honestB := Code("B", "A")
	if honestA != honestB {
		t.Fatal("honest codes must match")
	}
	mitmA := Code("A", "M")
	mitmB := Code("B", "M")
	if mitmA == mitmB {
		t.Fatal("a man-in-the-middle must produce different codes on each side")
	}
}
