package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"hop/internal/merge"
	"hop/internal/syncer"
)

/*
UIResolver resolves a diverged session by showing a compact summary of the two
transcripts and prompting the user to keep one side or abort.
*/
type UIResolver struct{}

/* Resolve renders the divergence summary for a session and prompts for a choice. */
func (UIResolver) Resolve(id string, local, incoming []byte) (syncer.Resolution, error) {
	s := merge.Summarize(local, incoming)
	fmt.Println(renderSummary(id, s))
	choice := "abort"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Session conflict: %s — choose", id)).
				Options(
					huh.NewOption("keep mine (local)", "local"),
					huh.NewOption("keep remote (incoming)", "remote"),
					huh.NewOption("abort", "abort"),
				).Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return syncer.Abort, err
	}
	switch choice {
	case "local":
		return syncer.KeepLocal, nil
	case "remote":
		return syncer.KeepIncoming, nil
	default:
		return syncer.Abort, nil
	}
}

/* renderSummary formats a DiffSummary into a short human-readable block. */
func renderSummary(id string, s merge.DiffSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "session conflict in %s: identical for %d line(s), then forks\n", id, s.ForkLine)
	fmt.Fprintf(&b, "  local    (%d lines): %s\n", s.LocalLines, strings.Join(s.LocalTail, " | "))
	fmt.Fprintf(&b, "  incoming (%d lines): %s", s.IncomingLines, strings.Join(s.IncomingTail, " | "))
	return boxStyle.Render(b.String())
}
