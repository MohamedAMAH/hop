package ui

import "github.com/charmbracelet/huh"

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
			huh.NewInput().Title("Project ID").Value(&v.ProjectID),
			huh.NewInput().Title("This machine's name").Value(&v.Machine),
			huh.NewInput().Title("Project path here").Value(&v.Path),
			huh.NewSelect[string]().Title("Transport").
				Options(
					huh.NewOption("folder", "folder"),
					huh.NewOption("ssh (coming soon)", "ssh"),
					huh.NewOption("cloud (coming soon)", "cloud"),
				).Value(&v.Transport),
		),
		huh.NewGroup(
			huh.NewInput().Title("Shared folder directory").Value(&v.Folder),
		).WithHideFunc(func() bool { return v.Transport != "folder" }),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Hand-off").
				Options(
					huh.NewOption("manual", "manual"),
					huh.NewOption("auto (coming soon)", "auto"),
				).Value(&v.Handoff),
		),
	)
	if err := form.Run(); err != nil {
		return InitValues{}, err
	}
	return v, nil
}
