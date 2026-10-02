package main

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
)

const defaultPresetDir = "~/Music/Audio Music Apps/Channel Strip Settings"

// Presets indexes Logic channel strip settings (.cst) by kind ("Bus", "Track") and normalized name.
type Presets struct {
	Dir   string
	files map[string]string // "bus/smashed guitars" -> "Bus/Smashed Guitars.cst"
	index map[string]bool   // library-relative paths from a preset index, when the library isn't on this machine
}

// presetsFor uses the live library when it exists, otherwise the committed preset index (e.g. in a container).
func presetsFor(p Paths, cfg Config) Presets {
	lib := loadPresets(cmp.Or(cfg.PresetDir, defaultPresetDir))
	if _, err := os.Stat(lib.Dir); err == nil || p.PresetIndex == "" {
		return lib
	}
	return loadPresetIndex(lib.Dir, p.PresetIndex)
}

// loadPresetIndex reads "Channel Strip Settings/Bus/x.cst" and "Plug-In Settings/<plugin>/y.pst" lines.
func loadPresetIndex(dir, path string) Presets {
	p := Presets{Dir: dir, files: map[string]string{}, index: map[string]bool{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		p.index[line] = true
		rel, ok := strings.CutPrefix(line, "Channel Strip Settings/")
		if !ok {
			continue
		}
		if kind, file, _ := strings.Cut(rel, "/"); (kind == "Bus" || kind == "Track") && strings.HasSuffix(file, ".cst") {
			p.files[presetKey(kind, strings.TrimSuffix(file, ".cst"))] = rel
		}
	}
	return p
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
	if p.index != nil {
		return p.index["Channel Strip Settings/"+rel]
	}
	_, err := os.Stat(filepath.Join(p.Dir, rel))
	return err == nil
}

// pluginPresetExists checks "<plugin>/<name>.pst" in the sibling Plug-In Settings folder.
func (p Presets) pluginPresetExists(rel string) bool {
	if p.index != nil {
		return p.index["Plug-In Settings/"+rel]
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(p.Dir), "Plug-In Settings", rel))
	return err == nil
}
