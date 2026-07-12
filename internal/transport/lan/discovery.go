package lan

import (
	"context"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/grandcat/zeroconf"
)

/* serviceType is the mDNS service hop advertises and browses. */
const serviceType = "_hop._tcp"

/* Discovered is a machine seen on the LAN (or entered manually). */
type Discovered struct {
	Name        string
	Address     string
	Fingerprint string
}

/*
Advertise announces this machine on the LAN with its name, fingerprint (in a TXT
record), and listening port, until the returned closer is closed.
*/
func Advertise(name, fingerprint string, port int) (io.Closer, error) {
	server, err := zeroconf.Register(name, serviceType, "local.", port,
		[]string{"fp=" + fingerprint}, nil)
	if err != nil {
		return nil, err
	}
	return closerFunc(server.Shutdown), nil
}

/* Browse lists hop machines currently advertising on the LAN within the timeout. */
func Browse(ctx context.Context, timeout time.Duration) ([]Discovered, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	entries := make(chan *zeroconf.ServiceEntry)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(cctx, serviceType, "local.", entries); err != nil {
		return nil, err
	}
	var out []Discovered
	for e := range entries {
		d := Discovered{Name: e.Instance}
		if len(e.AddrIPv4) > 0 {
			d.Address = net.JoinHostPort(e.AddrIPv4[0].String(), strconv.Itoa(e.Port))
		}
		for _, txt := range e.Text {
			if len(txt) > 3 && txt[:3] == "fp=" {
				d.Fingerprint = txt[3:]
			}
		}
		out = append(out, d)
	}
	return out, nil
}

/* ManualPeer builds a Discovered from a name and an explicit address. */
func ManualPeer(name, address string) Discovered { return Discovered{Name: name, Address: address} }

/* closerFunc adapts a func to io.Closer. */
type closerFunc func()

/* Close runs the underlying shutdown function. */
func (f closerFunc) Close() error { f(); return nil }
