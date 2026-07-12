package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop/internal/agent/claude"
	"hop/internal/config"
	"hop/internal/osinfo"
	"hop/internal/syncer"
	"hop/internal/transport/lan"
)

/* lanDir returns hop's LAN state directory (identity + peers). */
func lanDir() (string, error) {
	dir, err := config.DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lan"), nil
}

/* lanIdentityAndPeers loads (or creates) this machine's LAN identity and peer store. */
func lanIdentityAndPeers() (lan.Identity, *lan.Peers, string, error) {
	dir, err := lanDir()
	if err != nil {
		return lan.Identity{}, nil, "", err
	}
	id, err := lan.LoadOrCreateIdentity(dir)
	if err != nil {
		return lan.Identity{}, nil, "", err
	}
	peersPath := filepath.Join(dir, "peers.json")
	peers, err := lan.LoadPeers(peersPath)
	if err != nil {
		return lan.Identity{}, nil, "", err
	}
	return id, peers, peersPath, nil
}

/* lanDepsFunc builds a DepsFunc that assembles syncer.Deps for a project from local config. */
func lanDepsFunc(cfg config.Config, home, stateDir string) lan.DepsFunc {
	return func(projectID string) (syncer.Deps, error) {
		if _, ok := cfg.Projects[projectID]; !ok {
			return syncer.Deps{}, fmt.Errorf("project %q is not configured on this machine", projectID)
		}
		return syncer.Deps{Cfg: cfg, Agent: claude.New(), Home: home, StateDir: stateDir, OS: osinfo.Current()}, nil
	}
}

/* cmdLan dispatches the LAN subcommands. */
func cmdLan(args []string, interactive bool) error {
	if len(args) < 1 {
		return errors.New("usage: hop lan <serve|link|sync|peers> [flags]")
	}
	switch args[0] {
	case "serve":
		return lanServe(args[1:])
	case "link":
		return lanLink(args[1:])
	case "sync":
		return lanSync(args[1:])
	case "peers":
		return lanPeers()
	default:
		return fmt.Errorf("unknown lan subcommand %q", args[0])
	}
}

/* lanServe enters ready mode: advertises via mDNS and serves the peer service until interrupted. */
func lanServe(args []string) error {
	fs := flag.NewFlagSet("lan serve", flag.ExitOnError)
	bind := fs.String("bind", "0.0.0.0:0", "address to listen on")
	fs.Parse(args)

	id, peers, peersPath, err := lanIdentityAndPeers()
	if err != nil {
		return err
	}
	cfgPath, stateDir, home, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	tcpLn, err := net.Listen("tcp", *bind)
	if err != nil {
		return err
	}
	defer tcpLn.Close()
	addr := tcpLn.Addr().String()
	svc := lan.NewService(id, peers, peersPath, cfg.Machine, addr, lanDepsFunc(cfg, home, stateDir))
	ln := tls.NewListener(tcpLn, svc.ServerTLSConfig())
	fmt.Printf("hop LAN ready on %s\n  device fingerprint: %s\n  (other machines can use this address for manual linking)\n", addr, id.Fingerprint)
	if adv, aerr := lan.Advertise(cfg.Machine, id.Fingerprint, tcpLn.Addr().(*net.TCPAddr).Port); aerr == nil {
		defer adv.Close()
	} else {
		fmt.Println("note: mDNS advertising unavailable; use the address above for manual linking")
	}
	go acceptPendingLoop(svc)
	return (&http.Server{Handler: svc.Handler()}).Serve(ln)
}

/*
acceptPendingLoop polls the service for pairing requests awaiting confirmation
and prompts the operator on stdin to accept or reject each one by its code.
*/
func acceptPendingLoop(svc *lan.Service) {
	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for _, p := range svc.Pending() {
			fmt.Printf("\npairing request from %q at %s\n  confirmation code: %s\n", p.Name, p.Address, p.Code)
			fmt.Printf("match this code on %s? [y/N]: ", p.Name)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "y") || strings.HasPrefix(line, "Y") {
				if err := svc.Confirm(p.Fingerprint); err != nil {
					fmt.Println("error:", err)
				} else {
					fmt.Printf("linked with %s\n", p.Name)
				}
			} else {
				svc.Reject(p.Fingerprint)
				fmt.Printf("rejected pairing request from %s\n", p.Name)
			}
		}
	}
}

/* lanLink pairs this machine with a peer at a given address, confirming the code interactively. */
func lanLink(args []string) error {
	fs := flag.NewFlagSet("lan link", flag.ExitOnError)
	peerAddr := fs.String("peer", "", "the peer's host:port")
	listen := fs.String("listen", "", "this machine's reachable address, for the peer's future pulls")
	fs.Parse(args)
	if *peerAddr == "" {
		return errors.New("lan link requires -peer")
	}

	id, peers, peersPath, err := lanIdentityAndPeers()
	if err != nil {
		return err
	}
	cfgPath, _, _, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	res, err := lan.Pair(id, cfg.Machine, *listen, *peerAddr)
	if err != nil {
		return err
	}
	fmt.Printf("pairing with %q\n  fingerprint: %s\n  confirmation code: %s\n", res.PeerName, res.PeerFingerprint, res.Code)
	fmt.Print("does this code match on the other machine? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if !(strings.HasPrefix(line, "y") || strings.HasPrefix(line, "Y")) {
		fmt.Println("linking cancelled")
		return nil
	}
	peers.Upsert(lan.Peer{Name: res.PeerName, Fingerprint: res.PeerFingerprint, LastAddress: res.PeerAddress})
	if err := peers.Save(peersPath); err != nil {
		return err
	}
	fmt.Printf("linked with %s\n", res.PeerName)
	return nil
}

/* lanSync pushes or pulls a project directly with a linked peer. */
func lanSync(args []string) error {
	fs := flag.NewFlagSet("lan sync", flag.ExitOnError)
	project := fs.String("project", "", "project ID")
	peerRef := fs.String("peer", "", "linked machine's fingerprint or name")
	pull := fs.Bool("pull", false, "pull instead of push")
	fs.Parse(args)
	if *project == "" || *peerRef == "" {
		return errors.New("lan sync requires -project and -peer")
	}

	id, peers, _, err := lanIdentityAndPeers()
	if err != nil {
		return err
	}
	cfgPath, stateDir, home, err := paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	peer, ok := peers.ByFingerprint(*peerRef)
	if !ok {
		for _, p := range peers.All() {
			if p.Name == *peerRef {
				peer, ok = p, true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("no linked machine matching %q (run hop lan link first)", *peerRef)
	}

	d, err := lanDepsFunc(cfg, home, stateDir)(*project)
	if err != nil {
		return err
	}
	d.Transport = lan.NewTransport(id, peer)

	var rep syncer.Report
	op := "push"
	if *pull {
		op = "pull"
		rep, err = syncer.Pull(d, *project, nowRFC3339(), syncer.AbortResolver{})
	} else {
		rep, err = syncer.Push(d, *project, nowRFC3339())
	}
	if err != nil {
		return err
	}
	fmt.Printf("lan %s ok: %d session(s), sequence %d\n", op, rep.Sessions, rep.Sequence)
	return nil
}

/* lanPeers prints every machine this one has linked with. */
func lanPeers() error {
	_, peers, _, err := lanIdentityAndPeers()
	if err != nil {
		return err
	}
	all := peers.All()
	if len(all) == 0 {
		fmt.Println("no linked machines yet")
		return nil
	}
	for _, p := range all {
		fmt.Printf("%s  %s  %s\n", p.Name, p.Fingerprint, p.LastAddress)
	}
	return nil
}
