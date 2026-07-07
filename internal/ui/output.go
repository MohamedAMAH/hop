package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	okStyle    = lipgloss.NewStyle().Bold(true)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	keyStyle   = lipgloss.NewStyle().Faint(true)
)

/* RenderSync formats a push/pull result box with the session count and sequence. */
func RenderSync(op string, sessions, sequence int) string {
	line := fmt.Sprintf("%s %s · %d session(s) · sequence %d",
		okStyle.Render("✓"), titleStyle.Render(op), sessions, sequence)
	return boxStyle.Render(line)
}

/* RenderStatus formats the static status panel. */
func RenderStatus(machine, project, transport, handoff string, sequence int, batonHeld bool, lastSync string) string {
	rows := []string{
		kv("machine", machine),
		kv("project", project),
		kv("transport", transport),
		kv("handoff", handoff),
		kv("sequence", fmt.Sprintf("%d", sequence)),
		kv("baton", fmt.Sprintf("%v (local only in Plan 1)", batonHeld)),
		kv("last sync", lastSync),
	}
	return boxStyle.Render(strings.Join(rows, "\n"))
}

/* RenderMessage formats a one-line labelled message (e.g. "note", "error"). */
func RenderMessage(kind, msg string) string {
	return fmt.Sprintf("%s %s", keyStyle.Render(kind+":"), msg)
}

/* kv formats a faint key and its value on one line. */
func kv(k, v string) string { return fmt.Sprintf("%s %s", keyStyle.Render(k+":"), v) }
