// Package presets finds Logic channel strip (.cst) and plugin (.pst) presets, from the library or a committed index.
package presets

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"

	"studio/engine/internal/config"
)

const DefaultDir = "~/Music/Audio Music Apps/Channel Strip Settings"

// Presets indexes Logic channel strip settings (.cst) by kind ("Bus", "Track") and normalized name.
type Presets struct {
	Dir   string
	files map[string]string // "bus/smashed guitars" -> "Bus/Smashed Guitars.cst"
	index map[string]bool   // library-relative paths from a preset index, when the library isn't on this machine
}

// For uses the live library when it exists, otherwise the committed preset index (e.g. in a container).
func For(p config.Paths, cfg config.Config) Presets {
	lib := Load(cmp.Or(cfg.PresetDir, DefaultDir))
	if _, err := os.Stat(lib.Dir); err == nil || p.PresetIndex == "" {
		return lib
	}
	return LoadIndex(lib.Dir, p.PresetIndex)
}

// LoadIndex reads "Channel Strip Settings/Bus/x.cst" and "Plug-In Settings/<plugin>/y.pst" lines.
func LoadIndex(dir, path string) Presets {
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
			p.files[key(kind, strings.TrimSuffix(file, ".cst"))] = rel
		}
	}
	return p
}

func key(kind, name string) string {
	return strings.ToLower(kind + "/" + strings.ReplaceAll(name, "_", " "))
}

// Load indexes a Channel Strip Settings folder ("~/" allowed).
func Load(dir string) Presets {
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	p := Presets{Dir: dir, files: map[string]string{}}
	for _, kind := range []string{"Bus", "Track"} {
		matches, _ := filepath.Glob(filepath.Join(dir, kind, "*.cst"))
		for _, m := range matches {
			rel := kind + "/" + filepath.Base(m)
			p.files[key(kind, strings.TrimSuffix(filepath.Base(m), ".cst"))] = rel
		}
	}
	return p
}

// Find matches a channel strip setting by name: "Smashed_Guitars" finds "Bus/Smashed Guitars.cst".
func (p Presets) Find(kind, name string) (string, bool) {
	rel, ok := p.files[key(kind, name)]
	return rel, ok
}

func (p Presets) Exists(rel string) bool {
	if p.index != nil {
		return p.index["Channel Strip Settings/"+rel]
	}
	_, err := os.Stat(filepath.Join(p.Dir, rel))
	return err == nil
}

// PluginPresetExists checks "<plugin>/<name>.pst" in the sibling Plug-In Settings folder.
func (p Presets) PluginPresetExists(rel string) bool {
	if p.index != nil {
		return p.index["Plug-In Settings/"+rel]
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(p.Dir), "Plug-In Settings", rel))
	return err == nil
}
