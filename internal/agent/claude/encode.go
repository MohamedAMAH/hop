/* Package claude adapts hop to claude-code's on-disk session storage. */
package claude

import "strings"

/*
	EncodeDir converts an absolute project path into claude-code's project

directory name: every byte that is not [A-Za-z0-9] becomes '-'. Case and
digits are preserved and consecutive dashes are NOT collapsed. The mapping
is lossy and MUST NOT be inverted.
*/
func EncodeDir(projectRoot string) string {
	var b strings.Builder
	b.Grow(len(projectRoot))
	for _, r := range projectRoot {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
