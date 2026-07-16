package syncer

import (
	"hop/internal/bundle"
	"hop/internal/osinfo"
	"hop/internal/rewrite"
)

/* safeArtifactPath delegates to bundle.ValidFilePath, the single source of truth for bundle file-path validation. */
func safeArtifactPath(relPath string) error {
	return bundle.ValidFilePath(relPath)
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
