package lan

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

/*
Code returns a 6-digit confirmation code derived from two fingerprints in sorted
order, so both machines compute the same code from the certificates they saw on
the TLS channel. A man-in-the-middle presents a different certificate to each
side and therefore produces a different code on each screen.
*/
func Code(localFP, peerFP string) string {
	lo, hi := localFP, peerFP
	if hi < lo {
		lo, hi = hi, lo
	}
	sum := sha256.Sum256([]byte(lo + "|" + hi))
	n := binary.BigEndian.Uint32(sum[:4]) % 1000000
	return fmt.Sprintf("%06d", n)
}
