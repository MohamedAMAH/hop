/*
Package rewrite neutralizes and materializes machine-specific paths inside
claude-code JSONL transcripts, operating on decoded JSON string values so
escaping and structure are preserved exactly.
*/
package rewrite

import (
	"bytes"
	"encoding/json"
	"strings"
)

/*
rewriteJSONStrings scans src for JSON string literals, decodes each, applies
fn to the decoded value, and re-encodes it without HTML-escaping. All bytes
outside string literals are copied verbatim, so key order, whitespace,
numbers, and the newlines between JSONL records are preserved.
*/
func rewriteJSONStrings(src []byte, fn func(string) string) ([]byte, error) {
	var out strings.Builder
	out.Grow(len(src))
	i := 0
	for i < len(src) {
		c := src[i]
		if c != '"' {
			out.WriteByte(c)
			i++
			continue
		}
		// Find the end of this string literal, respecting backslash escapes.
		j := i + 1
		for j < len(src) {
			if src[j] == '\\' {
				j += 2
				continue
			}
			if src[j] == '"' {
				break
			}
			j++
		}
		if j >= len(src) {
			// Unterminated string; emit remainder verbatim.
			out.Write(src[i:])
			break
		}
		literal := src[i : j+1] // includes both quotes
		var decoded string
		if err := json.Unmarshal(literal, &decoded); err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(fn(decoded)); err != nil {
			return nil, err
		}
		out.Write(bytes.TrimRight(buf.Bytes(), "\n"))
		i = j + 1
	}
	return []byte(out.String()), nil
}
