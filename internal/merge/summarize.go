package merge

import "strings"

/*
DiffSummary is a compact, UI-free description of how two transcripts differ:
the number of identical leading lines, each side's line count, and up to three
trailing lines unique to each side after the fork point.
*/
type DiffSummary struct {
	ForkLine      int
	LocalLines    int
	IncomingLines int
	LocalTail     []string
	IncomingTail  []string
}

/*
Summarize compares two transcripts line by line and reports where they first
differ and the trailing lines unique to each after that point.
*/
func Summarize(local, incoming []byte) DiffSummary {
	l := splitLines(local)
	in := splitLines(incoming)
	fork := 0
	for fork < len(l) && fork < len(in) && l[fork] == in[fork] {
		fork++
	}
	return DiffSummary{
		ForkLine:      fork,
		LocalLines:    len(l),
		IncomingLines: len(in),
		LocalTail:     lastN(l[fork:], 3),
		IncomingTail:  lastN(in[fork:], 3),
	}
}

/* splitLines splits transcript bytes into lines, dropping a single trailing newline's empty element. */
func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := strings.Split(string(b), "\n")
	if len(s) > 0 && s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	return s
}

/* lastN returns the final n elements of s, or all of them when there are fewer. */
func lastN(s []string, n int) []string {
	if len(s) <= n {
		if len(s) == 0 {
			return nil
		}
		return s
	}
	return s[len(s)-n:]
}
