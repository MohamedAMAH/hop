package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

/*
MenuChoice identifies the action a user picked on the home menu.
*/
type MenuChoice int

const (
	MenuNone   MenuChoice = iota // No choice was made (the menu was dismissed).
	MenuSetup                    // Set up or link a project.
	MenuPush                     // Push this machine's sessions.
	MenuPull                     // Pull the other machine's sessions.
	MenuStatus                   // Show a project's status.
	MenuConfig                   // Edit a project's config.
	MenuQuit                     // Leave the menu.
)

/* menuItem is one selectable row on the home menu. */
type menuItem struct {
	key    string
	label  string
	desc   string
	choice MenuChoice
}

/* menuItems is the fixed list of home-menu rows, in display order. */
var menuItems = []menuItem{
	{"1", "Set up / link a project", "", MenuSetup},
	{"2", "Push", "send this machine's sessions", MenuPush},
	{"3", "Pull", "bring in the other machine's sessions", MenuPull},
	{"4", "Status", "", MenuStatus},
	{"5", "Edit a project's config", "", MenuConfig},
	{"0", "Quit", "", MenuQuit},
}

/* banner is the ASCII title shown at the top of the home menu. */
const banner = `██   ██  ██████  ██████
██   ██ ██    ██ ██   ██
███████ ██    ██ ██████
██   ██ ██    ██ ██
██   ██  ██████  ██`

var (
	menuBannerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 2)
	menuSubtitle    = lipgloss.NewStyle().Faint(true)
	menuSelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	menuKeyStyle    = lipgloss.NewStyle().Faint(true)
	menuDescStyle   = lipgloss.NewStyle().Faint(true)
	menuHelpStyle   = lipgloss.NewStyle().Faint(true)
)

/* menuModel is the Bubble Tea model backing the home menu. */
type menuModel struct {
	cursor int
	choice MenuChoice
}

/* Init satisfies tea.Model and starts with no pending command. */
func (m menuModel) Init() tea.Cmd { return nil }

/* Update handles arrow, number, select, and quit keys for the home menu. */
func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q", "esc":
		m.choice = MenuQuit
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(menuItems)-1 {
			m.cursor++
		}
	case "enter", " ":
		m.choice = menuItems[m.cursor].choice
		return m, tea.Quit
	default:
		// A number key jumps to and selects its row.
		for i, it := range menuItems {
			if key.String() == it.key {
				m.cursor = i
				m.choice = it.choice
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

/* Banner returns the styled HOP title box shown at the top of every screen. */
func Banner() string {
	head := banner + "\n" + menuSubtitle.Render("sync your agent sessions across machines")
	return menuBannerStyle.Render(head)
}

/* View renders the banner, the selectable rows, and the key help. */
func (m menuModel) View() string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n\n")
	for i, it := range menuItems {
		marker := "  "
		row := fmt.Sprintf("%s  %s", menuKeyStyle.Render(it.key), it.label)
		if it.desc != "" {
			row += "  " + menuDescStyle.Render("— "+it.desc)
		}
		if i == m.cursor {
			marker = menuSelStyle.Render("▸ ")
			row = menuSelStyle.Render(fmt.Sprintf("%s  %s", it.key, it.label))
			if it.desc != "" {
				row += "  " + menuDescStyle.Render("— "+it.desc)
			}
		}
		b.WriteString("  " + marker + row + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + menuHelpStyle.Render("↑/↓ or a number to move · Enter to select · q to quit"))
	b.WriteString("\n")
	return b.String()
}

/*
RunMenu shows the full-screen home menu and returns the chosen action. It must
be called only on an interactive terminal.
*/
func RunMenu() (MenuChoice, error) {
	final, err := tea.NewProgram(menuModel{}, tea.WithAltScreen()).Run()
	if err != nil {
		return MenuNone, err
	}
	return final.(menuModel).choice, nil
}

/*
PickProject prompts the user to choose one of the given project IDs. It returns
the single ID directly when only one is configured, and an empty string when the
list is empty.
*/
func PickProject(ids []string) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	choice := ids[0]
	opts := make([]huh.Option[string], len(ids))
	for i, id := range ids {
		opts[i] = huh.NewOption(id, id)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Choose a project").Options(opts...).Value(&choice),
	)).WithTheme(hopTheme())
	if err := form.Run(); err != nil {
		return "", err
	}
	return choice, nil
}

/* Pause waits for the user to press Enter so a command's output stays visible. */
func Pause() {
	fmt.Print("\n  press Enter to return to the menu…")
	fmt.Scanln()
}
