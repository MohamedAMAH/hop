package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

/*
hopTheme is the shared Huh theme applied to every interactive form so all
screens match: a pink accent on titles and selections and a rounded border on
the focused field, echoing the home menu.
*/
func hopTheme() *huh.Theme {
	accent := lipgloss.Color("212")
	dim := lipgloss.Color("241")
	t := huh.ThemeBase()
	t.Focused.Base = t.Focused.Base.BorderStyle(lipgloss.RoundedBorder()).BorderForeground(accent)
	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)
	t.Focused.Description = t.Focused.Description.Foreground(dim)
	t.Blurred.Title = t.Blurred.Title.Foreground(dim)
	return t
}

/*
InitValues holds the fields the init/config form collects. The CLI maps these
onto the stored config, overwriting only what changed.
*/
type InitValues struct {
	ProjectID string
	Machine   string
	Path      string
	Transport string
	Folder    string
	Handoff   string
}

/*
RunInitForm presents a guided form seeded with existing values and returns the
edited values. It must only be called on an interactive terminal.
*/
func RunInitForm(existing InitValues) (InitValues, error) {
	v := existing
	if v.Transport == "" {
		v.Transport = "folder"
	}
	if v.Handoff == "" {
		v.Handoff = "manual"
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What's the project name?").
				Description("A short ID shared across your machines, e.g. hop.").
				Value(&v.ProjectID),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("What's this machine's name?").
				Description("How this laptop is identified, e.g. laptop-A or nixos.").
				Value(&v.Machine),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Where does this project live on this machine?").
				Description("The project's absolute path here.").
				Value(&v.Path),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How should sessions travel between machines?").
				Options(
					huh.NewOption("folder", "folder"),
					huh.NewOption("ssh (coming soon)", "ssh"),
					huh.NewOption("cloud (coming soon)", "cloud"),
				).Value(&v.Transport),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Which shared folder should hop use?").
				Description("A directory kept in sync across machines (Syncthing, Dropbox, …).").
				Value(&v.Folder),
		).WithHideFunc(func() bool { return v.Transport != "folder" }),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How do you want hand-off to work?").
				Options(
					huh.NewOption("manual", "manual"),
					huh.NewOption("auto (coming soon)", "auto"),
				).Value(&v.Handoff),
		),
	).WithTheme(hopTheme())
	if err := form.Run(); err != nil {
		return InitValues{}, err
	}
	return v, nil
}
