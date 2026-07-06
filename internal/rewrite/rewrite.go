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

	"hop/internal/osinfo"
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

/* isBoundary reports whether r can precede a path, so a root match starting right after it is anchored. */
func isBoundary(r byte) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '\'', '=', '(', ')', '[', ']', '{', '}',
		',', ';', ':', '<', '>', '`', '|':
		return true
	}
	return false
}

/* isPathTail reports whether r may appear inside a path tail that follows a matched root. */
func isPathTail(r byte) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '\'', '`', '|', '<', '>':
		return false
	}
	return true
}

/*
Neutralize replaces every anchored occurrence of sourceRoot inside jsonl's
decoded JSON string values with token, normalizing the following path tail's
separators to '/'. An occurrence is anchored when it starts at a path
boundary and ends at the string's end, a path separator, or another
boundary rune, so it never matches a longer sibling path segment that
merely shares sourceRoot as a prefix. Matching is case-insensitive when
sourceOS is Windows.
*/
func Neutralize(jsonl []byte, sourceRoot string, sourceOS osinfo.OS, token string) ([]byte, error) {
	root := sourceRoot
	caseless := sourceOS == osinfo.Windows
	return rewriteJSONStrings(jsonl, func(s string) string {
		var b strings.Builder
		i := 0
		for i < len(s) {
			atBoundary := i == 0 || isBoundary(s[i-1])
			end := i + len(root)
			if atBoundary && end <= len(s) && rootMatches(s[i:end], root, caseless) && rootEndsHere(s, end) {
				b.WriteString(token)
				i = end
				// Normalize the contiguous path tail's separators to '/'.
				for i < len(s) && isPathTail(s[i]) {
					if s[i] == '\\' {
						b.WriteByte('/')
					} else {
						b.WriteByte(s[i])
					}
					i++
				}
				continue
			}
			b.WriteByte(s[i])
			i++
		}
		return b.String()
	})
}

/* rootMatches reports whether candidate equals root, ignoring case when caseless is true. */
func rootMatches(candidate, root string, caseless bool) bool {
	if caseless {
		return strings.EqualFold(candidate, root)
	}
	return candidate == root
}

/*
rootEndsHere reports whether a root match ending at index end in s stops at
a genuine path boundary: the end of the string, a path separator, or a
delimiter rune, rather than continuing into a longer path segment.
*/
func rootEndsHere(s string, end int) bool {
	if end == len(s) {
		return true
	}
	c := s[end]
	return c == '/' || c == '\\' || isBoundary(c)
}

/*
Materialize replaces every occurrence of token inside jsonl's decoded JSON
string values with targetRoot, converting the following path tail's '/'
separators to targetOS's separator.
*/
func Materialize(jsonl []byte, targetRoot string, targetOS osinfo.OS, token string) ([]byte, error) {
	sep := targetOS.Sep()
	return rewriteJSONStrings(jsonl, func(s string) string {
		var b strings.Builder
		i := 0
		for i < len(s) {
			if strings.HasPrefix(s[i:], token) {
				b.WriteString(targetRoot)
				i += len(token)
				// Convert the contiguous path tail's separators to the target OS's separator.
				for i < len(s) && isPathTail(s[i]) {
					if s[i] == '/' {
						b.WriteString(sep)
					} else {
						b.WriteByte(s[i])
					}
					i++
				}
				continue
			}
			b.WriteByte(s[i])
			i++
		}
		return b.String()
	})
}
