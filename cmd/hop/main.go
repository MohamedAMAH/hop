/* Command hop syncs claude-code sessions across machines. */
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"hop/internal/agent/claude"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/state"
	"hop/internal/syncer"
	"hop/internal/transport"
	"hop/internal/transport/folder"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "push":
		err = cmdSync(os.Args[2:], "push")
	case "pull":
		err = cmdSync(os.Args[2:], "pull")
	case "status":
		err = cmdStatus(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

/* usage prints the top-level command summary to stderr. */
func usage() {
	fmt.Fprintln(os.Stderr, "usage: hop <init|push|pull|status> [flags]")
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

/* cmdInit records this machine's path and transport settings for a project. */
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	machine := fs.String("machine", "", "this machine's name")
	path := fs.String("path", "", "absolute project path on this machine")
	tport := fs.String("transport", "folder", "transport (folder)")
	folderDir := fs.String("folder", "", "folder transport: the shared directory")
	handoff := fs.String("handoff", "manual", "hand-off mode (manual)")
	fs.Parse(args)
	if *project == "" || *machine == "" || *path == "" {
		return errors.New("init requires -project, -machine, and -path")
	}
	if *tport == "folder" && *folderDir == "" {
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
	cfg.Machine = *machine
	p, ok := cfg.Projects[*project]
	if !ok {
		p = config.Project{Paths: map[string]string{}, TransportConfig: map[string]string{}}
	}
	if p.Paths == nil {
		p.Paths = map[string]string{}
	}
	if p.TransportConfig == nil {
		p.TransportConfig = map[string]string{}
	}
	p.Paths[*machine] = *path
	p.Transport = *tport
	p.Handoff = *handoff
	if *tport == "folder" {
		p.TransportConfig["dir"] = *folderDir
	}
	cfg.Projects[*project] = p
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}
	fmt.Printf("Configured project %q on machine %q at %s\n", *project, *machine, *path)
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
func cmdSync(args []string, op string) error {
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
	now := nowRFC3339()
	var rep syncer.Report
	if op == "push" {
		rep, err = syncer.Push(d, *project, now)
	} else {
		var r syncer.Resolver = syncer.AbortResolver{}
		if *force {
			r = syncer.ForceResolver{}
		}
		rep, err = syncer.Pull(d, *project, now, r)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s ok: %d session(s), sequence %d\n", op, rep.Sessions, rep.Sequence)
	return nil
}

/* cmdStatus prints this machine's configuration and, if given a project, its sync state. */
func cmdStatus(args []string) error {
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
