// Package presetstest builds throwaway Logic preset libraries for tests.
package presetstest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/engine/internal/presets"
)

// Library builds a fake "Audio Music Apps" folder; paths starting with "Plug-In Settings/" go to its sibling folder.
func Library(t *testing.T, files ...string) presets.Presets {
	t.Helper()
	root := t.TempDir()
	cst := filepath.Join(root, "Channel Strip Settings")
	_ = os.MkdirAll(cst, 0o755)
	for _, f := range files {
		path := filepath.Join(cst, f)
		if strings.HasPrefix(f, "Plug-In Settings/") {
			path = filepath.Join(root, f)
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, nil, 0o644)
	}
	return presets.Load(cst)
}
