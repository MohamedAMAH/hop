/*
Package ui owns hop's terminal presentation: terminal detection, styling, forms,
and the interactive divergence resolver. It is the only package that imports the
Charm libraries.
*/
package ui

import (
	"os"

	"golang.org/x/term"
)

/*
DetectTerminal reports whether stdin and stdout are both real terminals. It is a
variable so tests can substitute a deterministic check.
*/
var DetectTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

/*
Interactive reports whether hop should use its rich, interactive path. A true
plain flag forces the plain path regardless of the terminal.
*/
func Interactive(plain bool) bool {
	if plain {
		return false
	}
	return DetectTerminal()
}
