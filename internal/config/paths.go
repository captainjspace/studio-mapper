package config

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const Name = "studio_config.json"

// Paths locates the config and everything it points at, so studio-map runs from any directory.
type Paths struct {
	Config string // resolved config file
	Base   string // directory relative sheet/state paths are resolved against
	Sheet  string // the selected rig's sheet; set after rig selection
	State  string

	PresetIndex string // optional; used when the preset library isn't on this machine
}

// Abs resolves a config-relative path.
func (p Paths) Abs(rel string) string {
	if rel == "" || filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(p.Base, rel)
}

// candidates lists where the config is looked for, in order.
func candidates(flag string) []string {
	var c []string
	if flag != "" {
		return []string{flag}
	}
	if env := os.Getenv("STUDIO_MAP_CONFIG"); env != "" {
		return []string{env}
	}
	if dir, err := os.Getwd(); err == nil {
		for ; ; dir = filepath.Dir(dir) {
			c = append(c, filepath.Join(dir, Name))
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		c = append(c, filepath.Join(home, ".config", "studio-map", Name))
	}
	return c
}

// TakeFlag removes "--name value" (or "-name value") from args and returns the value.
func TakeFlag(args []string, name string) (string, []string) {
	for i, a := range args {
		if (a == "--"+name || a == "-"+name) && i+1 < len(args) {
			return args[i+1], slices.Delete(slices.Clone(args), i, i+2)
		}
	}
	return "", args
}

// Resolve finds the config (--config, $STUDIO_MAP_CONFIG, here or a parent folder, ~/.config/studio-map) and loads it.
func Resolve(flag string) (Paths, Config, error) {
	tried := candidates(flag)
	for _, path := range tried {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Paths{}, Config{}, err
		}
		c, err := Load(real)
		if err != nil {
			return Paths{}, Config{}, err
		}
		p := Paths{Config: real, Base: filepath.Dir(real)}
		p.State = p.Abs(cmp.Or(c.StateDir, "state"))
		p.PresetIndex = p.Abs(c.PresetIndex)
		return p, c, nil
	}
	return Paths{}, Config{}, fmt.Errorf("no %s found; looked in:\n  %s\nset --config <path> or STUDIO_MAP_CONFIG, or run `make install` to link ~/.config/studio-map/%s",
		Name, strings.Join(tried, "\n  "), Name)
}
