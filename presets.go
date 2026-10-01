package main

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultPresetDir = "~/Music/Audio Music Apps/Channel Strip Settings"

// Presets indexes Logic channel strip settings (.cst) by kind ("Bus", "Track") and normalized name.
type Presets struct {
	Dir   string
	files map[string]string // "bus/smashed guitars" -> "Bus/Smashed Guitars.cst"
}

func presetKey(kind, name string) string {
	return strings.ToLower(kind + "/" + strings.ReplaceAll(name, "_", " "))
}

func loadPresets(dir string) Presets {
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	p := Presets{Dir: dir, files: map[string]string{}}
	for _, kind := range []string{"Bus", "Track"} {
		matches, _ := filepath.Glob(filepath.Join(dir, kind, "*.cst"))
		for _, m := range matches {
			rel := kind + "/" + filepath.Base(m)
			p.files[presetKey(kind, strings.TrimSuffix(filepath.Base(m), ".cst"))] = rel
		}
	}
	return p
}

func (p Presets) find(kind, name string) (string, bool) {
	rel, ok := p.files[presetKey(kind, name)]
	return rel, ok
}

func (p Presets) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(p.Dir, rel))
	return err == nil
}

// pluginPresetExists checks "<plugin>/<name>.pst" in the sibling Plug-In Settings folder.
func (p Presets) pluginPresetExists(rel string) bool {
	_, err := os.Stat(filepath.Join(filepath.Dir(p.Dir), "Plug-In Settings", rel))
	return err == nil
}
