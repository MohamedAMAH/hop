/* Command hop syncs claude-code sessions across machines. */
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"hop/internal/agent/claude"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/state"
	"hop/internal/syncer"
	"hop/internal/transport"
	"hop/internal/transport/folder"
	"hop/internal/ui"
)

func main() {
	args, plain := extractPlainFlag(os.Args[1:])
	interactive := ui.Interactive(plain)
	if len(args) < 1 {
		// Bare `hop` in a terminal opens the interactive home menu; otherwise
		// it prints usage so scripts still get a clear, non-blocking result.
		if interactive {
			if err := runMenu(); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
		usage()
		os.Exit(2)
	}
	var err error
	switch args[0] {
	case "init":
		err = cmdInit(args[1:], interactive)
	case "push":
		err = cmdSync(args[1:], "push", interactive)
	case "pull":
		err = cmdSync(args[1:], "pull", interactive)
	case "status":
		err = cmdStatus(args[1:], interactive)
	case "config":
		err = cmdConfig(args[1:], interactive)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

/*
extractPlainFlag removes a -plain/--plain flag from args, wherever it appears,
and reports whether it was present. The global flag is scanned ahead of
subcommand dispatch, so each subcommand's flag.FlagSet never sees it.
*/
func extractPlainFlag(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	plain := false
	for _, a := range args {
		if a == "-plain" || a == "--plain" {
			plain = true
			continue
		}
		out = append(out, a)
	}
	return out, plain
}

/* usage prints the top-level command summary to stderr. */
func usage() {
	fmt.Fprintln(os.Stderr, "usage: hop [--plain] <init|push|pull|status|config> [flags]")
}

/*
runMenu drives the interactive home menu: it shows the menu, runs the chosen
action, and loops until the user quits. Action errors are shown and the loop
continues rather than exiting.
*/
func runMenu() error {
	for {
		choice, err := ui.RunMenu()
		if err != nil {
			return err
		}
		if choice == ui.MenuQuit || choice == ui.MenuNone {
			return nil
		}
		// Every sub-screen opens under the same banner so the app feels of a piece.
		fmt.Println(ui.Banner())
		var actErr error
		switch choice {
		case ui.MenuSetup:
			actErr = cmdInit(nil, true)
		case ui.MenuPush:
			actErr = menuSync("push")
		case ui.MenuPull:
			actErr = menuSync("pull")
		case ui.MenuStatus:
			actErr = menuStatus()
		case ui.MenuConfig:
			actErr = menuConfig()
		}
		if actErr != nil {
			fmt.Println(ui.RenderMessage("error", actErr.Error()))
		}
		ui.Pause()
	}
}

/*
chooseProject asks the user which configured project to act on. It returns an
empty ID (and prints a hint) when no project is configured yet.
*/
func chooseProject() (string, error) {
	cfgPath, _, _, err := paths()
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(cfg.Projects))
	for id := range cfg.Projects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		fmt.Println(ui.RenderMessage("note", `no projects yet — choose "Set up / link a project" first`))
		return "", nil
	}
	return ui.PickProject(ids)
}

/* menuSync picks a project and runs a push or pull for it from the home menu. */
func menuSync(op string) error {
	proj, err := chooseProject()
	if err != nil || proj == "" {
		return err
	}
	return cmdSync([]string{"-project", proj}, op, true)
}

/* menuStatus picks a project and shows its status from the home menu. */
func menuStatus() error {
	proj, err := chooseProject()
	if err != nil || proj == "" {
		return err
	}
	return cmdStatus([]string{"-project", proj}, true)
}

/* menuConfig picks a project and opens its config form from the home menu. */
func menuConfig() error {
	proj, err := chooseProject()
	if err != nil || proj == "" {
		return err
	}
	return cmdConfig([]string{"-project", proj}, true)
}

/* paths returns hop's config file path, state directory, and the user's home directory. */
func paths() (cfgPath, stateDir, home string, err error) {
	dir, err := config.DefaultDir()
	if err != nil {
		return "", "", "", err
	}
	home, err = os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}
	return filepath.Join(dir, "config.json"), filepath.Join(dir, "state"), home, nil
}

/*
cmdInit records this machine's path and transport settings for a project. When
required flags are missing and the terminal is interactive, it seeds a guided
form with whatever flags were given instead of failing.
*/
func cmdInit(args []string, interactive bool) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	machine := fs.String("machine", "", "this machine's name")
	path := fs.String("path", "", "absolute project path on this machine")
	tport := fs.String("transport", "folder", "transport (folder)")
	folderDir := fs.String("folder", "", "folder transport: the shared directory")
	handoff := fs.String("handoff", "manual", "hand-off mode (manual)")
	fs.Parse(args)

	missing := *project == "" || *machine == "" || *path == ""
	if missing && !interactive {
		return errors.New("init requires -project, -machine, and -path")
	}

	values := ui.InitValues{
		ProjectID: *project,
		Machine:   *machine,
		Path:      *path,
		Transport: *tport,
		Folder:    *folderDir,
		Handoff:   *handoff,
	}
	if missing {
		var err error
		values, err = ui.RunInitForm(values)
		if err != nil {
			return err
		}
	}
	if values.ProjectID == "" || values.Machine == "" || values.Path == "" {
		return errors.New("init requires -project, -machine, and -path")
	}
	if values.Transport == "folder" && values.Folder == "" {
		return errors.New("folder transport requires -folder")
	}

	cfgPath, _, _, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg.Machine = values.Machine
	existing, ok := cfg.Projects[values.ProjectID]
	if !ok {
		existing = config.Project{Paths: map[string]string{}, TransportConfig: map[string]string{}}
	}
	updates := map[string]string{"transport": values.Transport, "handoff": values.Handoff}
	if values.Transport == "folder" {
		updates["folder"] = values.Folder
	}
	p := existing.WithUpdates(updates)
	if p.Paths == nil {
		p.Paths = map[string]string{}
	}
	p.Paths[values.Machine] = values.Path
	cfg.Projects[values.ProjectID] = p
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}
	msg := fmt.Sprintf("Configured project %q on machine %q at %s", values.ProjectID, values.Machine, values.Path)
	if interactive {
		fmt.Println(ui.RenderMessage("ok", msg))
	} else {
		fmt.Println(msg)
	}
	return nil
}

/* buildTransport constructs the transport configured for a project. */
func buildTransport(p config.Project) (transport.Transport, error) {
	switch p.Transport {
	case "folder":
		dir := p.TransportConfig["dir"]
		if dir == "" {
			return nil, errors.New("folder transport has no configured directory")
		}
		return folder.New(dir), nil
	default:
		return nil, fmt.Errorf("unknown transport %q", p.Transport)
	}
}

/* loadDeps assembles the syncer.Deps needed to push or pull a project. */
func loadDeps(project string) (syncer.Deps, error) {
	cfgPath, stateDir, home, err := paths()
	if err != nil {
		return syncer.Deps{}, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return syncer.Deps{}, err
	}
	p, ok := cfg.Projects[project]
	if !ok {
		return syncer.Deps{}, fmt.Errorf("project %q is not configured; run `hop init`", project)
	}
	tport, err := buildTransport(p)
	if err != nil {
		return syncer.Deps{}, err
	}
	return syncer.Deps{
		Cfg: cfg, Agent: claude.New(), Transport: tport,
		Home: home, StateDir: stateDir, OS: osinfo.Current(),
	}, nil
}

/* cmdSync runs a push or pull for a project and prints its report. */
func cmdSync(args []string, op string, interactive bool) error {
	fs := flag.NewFlagSet(op, flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	var force *bool
	if op == "pull" {
		force = fs.Bool("force", false, "override stale-pull / divergence warnings")
	}
	fs.Parse(args)
	if *project == "" {
		return errors.New(op + " requires -project")
	}
	d, err := loadDeps(*project)
	if err != nil {
		return err
	}
	if interactive {
		d.Notify = func(m string) { fmt.Println(ui.RenderMessage("note", m)) }
	} else {
		d.Notify = func(m string) { fmt.Fprintln(os.Stderr, "note: "+m) }
	}
	now := nowRFC3339()
	var rep syncer.Report
	if op == "push" {
		rep, err = syncer.Push(d, *project, now)
	} else {
		var r syncer.Resolver = syncer.AbortResolver{}
		if *force {
			r = syncer.ForceResolver{}
		} else if interactive {
			r = ui.UIResolver{}
		}
		rep, err = syncer.Pull(d, *project, now, r)
	}
	if err != nil {
		return err
	}
	if interactive {
		fmt.Println(ui.RenderSync(op, rep.Sessions, rep.Sequence))
	} else {
		fmt.Printf("%s ok: %d session(s), sequence %d\n", op, rep.Sessions, rep.Sequence)
	}
	return nil
}

/* cmdStatus prints this machine's configuration and, if given a project, its sync state. */
func cmdStatus(args []string, interactive bool) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	fs.Parse(args)
	cfgPath, stateDir, _, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if !interactive {
		fmt.Printf("machine: %s\n", cfg.Machine)
		if *project != "" {
			p := cfg.Projects[*project]
			fmt.Printf("project: %s\n  transport: %s\n  handoff: %s\n", *project, p.Transport, p.Handoff)
			st, _ := state.Load(filepath.Join(stateDir, *project+".json"))
			fmt.Printf("  last sequence: %d\n  baton held locally: %v (not enforced across machines in Plan 1)\n  last sync: %s\n",
				st.LastSyncedSequence, st.BatonHeld, st.LastSyncAt)
		}
		return nil
	}
	var p config.Project
	var st state.State
	if *project != "" {
		p = cfg.Projects[*project]
		st, _ = state.Load(filepath.Join(stateDir, *project+".json"))
	}
	fmt.Println(ui.RenderStatus(cfg.Machine, *project, p.Transport, p.Handoff, st.LastSyncedSequence, st.BatonHeld, st.LastSyncAt))
	return nil
}

/*
cmdConfig edits an already-configured project's settings. Unlike init, it
always starts from the existing project entry and applies only the fields
the flags or form actually changed, so unrelated settings are never wiped.
*/
func cmdConfig(args []string, interactive bool) error {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	machine := fs.String("machine", "", "this machine's name")
	path := fs.String("path", "", "absolute project path on this machine")
	tport := fs.String("transport", "", "transport (folder)")
	folderDir := fs.String("folder", "", "folder transport: the shared directory")
	handoff := fs.String("handoff", "", "hand-off mode (manual)")
	fs.Parse(args)
	if *project == "" {
		return errors.New("config requires -project")
	}

	cfgPath, _, _, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	existing, ok := cfg.Projects[*project]
	if !ok {
		return fmt.Errorf("project %q is not configured; run `hop init`", *project)
	}

	noFlags := *machine == "" && *path == "" && *tport == "" && *folderDir == "" && *handoff == ""
	if noFlags && !interactive {
		return errors.New("config requires at least one of -machine, -path, -transport, -folder, or -handoff")
	}

	values := ui.InitValues{
		ProjectID: *project,
		Machine:   *machine,
		Path:      *path,
		Transport: *tport,
		Folder:    *folderDir,
		Handoff:   *handoff,
	}
	if noFlags {
		// Seed the form with the current settings so unedited fields round-trip unchanged.
		seed := values
		seed.Machine = cfg.Machine
		seed.Transport = existing.Transport
		seed.Folder = existing.TransportConfig["dir"]
		seed.Handoff = existing.Handoff
		values, err = ui.RunInitForm(seed)
		if err != nil {
			return err
		}
	}

	updates := map[string]string{}
	if values.Transport != "" {
		updates["transport"] = values.Transport
	}
	if values.Handoff != "" {
		updates["handoff"] = values.Handoff
	}
	if values.Folder != "" {
		updates["folder"] = values.Folder
	}
	p := existing.WithUpdates(updates)
	machineName := values.Machine
	if machineName == "" {
		machineName = cfg.Machine
	}
	if values.Path != "" {
		if p.Paths == nil {
			p.Paths = map[string]string{}
		}
		p.Paths[machineName] = values.Path
	}
	cfg.Projects[*project] = p
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}
	msg := fmt.Sprintf("Updated project %q", *project)
	if interactive {
		fmt.Println(ui.RenderMessage("ok", msg))
	} else {
		fmt.Println(msg)
	}
	return nil
}
