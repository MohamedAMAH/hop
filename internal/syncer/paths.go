package syncer

import (
	"fmt"
	"path"
	"strings"

	"hop/internal/osinfo"
	"hop/internal/rewrite"
)

/*
safeArtifactPath rejects a bundle file path that is not a clean, relative,
in-tree path: absolute paths, drive-letter prefixes, any ".." segment, and
paths that do not survive path.Clean unchanged are refused so a bundle can
never write outside the project store.
*/
func safeArtifactPath(relPath string) error {
	if relPath == "" {
		return fmt.Errorf("hop: empty artifact path")
	}
	if strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, `\`) {
		return fmt.Errorf("hop: absolute artifact path %q", relPath)
	}
	if len(relPath) >= 2 && relPath[1] == ':' {
		return fmt.Errorf("hop: drive-letter artifact path %q", relPath)
	}
	if strings.Contains(relPath, `\`) {
		return fmt.Errorf("hop: backslash in artifact path %q", relPath)
	}
	if relPath != path.Clean(relPath) {
		return fmt.Errorf("hop: unclean artifact path %q", relPath)
	}
	for _, seg := range strings.Split(relPath, "/") {
		if seg == ".." {
			return fmt.Errorf("hop: traversal in artifact path %q", relPath)
		}
	}
	return nil
}

/*
neutralizeAll replaces both the project root and the storage prefix inside a
JSONL blob with their tokens. Each pass is skipped when its root or token is
empty, so callers may omit the storage prefix.
*/
func neutralizeAll(data []byte, root, storeDir string, os osinfo.OS, token, prefixToken string) ([]byte, error) {
	out := data
	var err error
	if storeDir != "" && prefixToken != "" {
		if out, err = rewrite.Neutralize(out, storeDir, os, prefixToken); err != nil {
			return nil, err
		}
	}
	if root != "" && token != "" {
		if out, err = rewrite.Neutralize(out, root, os, token); err != nil {
			return nil, err
		}
	}
	return out, nil
}

/* materializeAll reverses neutralizeAll, restoring the target machine's project root and storage prefix. */
func materializeAll(data []byte, root, storeDir string, os osinfo.OS, token, prefixToken string) ([]byte, error) {
	out := data
	var err error
	if root != "" && token != "" {
		if out, err = rewrite.Materialize(out, root, os, token); err != nil {
			return nil, err
		}
	}
	if storeDir != "" && prefixToken != "" {
		if out, err = rewrite.Materialize(out, storeDir, os, prefixToken); err != nil {
			return nil, err
		}
	}
	return out, nil
}
