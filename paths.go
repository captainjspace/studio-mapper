package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const configName = "studio_config.json"

// Paths locates the config and everything it points at, so studio-map runs from any directory.
type Paths struct {
	Config string // resolved config file
	Base   string // directory relative sheet/state paths are resolved against
	Sheet  string
	State  string
}

// configCandidates lists where the config is looked for, in order.
func configCandidates(flag string) []string {
	var c []string
	if flag != "" {
		return []string{flag}
	}
	if env := os.Getenv("STUDIO_MAP_CONFIG"); env != "" {
		return []string{env}
	}
	if dir, err := os.Getwd(); err == nil {
		for ; ; dir = filepath.Dir(dir) {
			c = append(c, filepath.Join(dir, configName))
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		c = append(c, filepath.Join(home, ".config", "studio-map", configName))
	}
	return c
}

// takeFlag removes "--name value" (or "-name value") from args and returns the value.
func takeFlag(args []string, name string) (string, []string) {
	for i, a := range args {
		if (a == "--"+name || a == "-"+name) && i+1 < len(args) {
			return args[i+1], slices.Delete(slices.Clone(args), i, i+2)
		}
	}
	return "", args
}

func resolvePaths(flag string, cfg func(path string) (Config, error)) (Paths, Config, error) {
	tried := configCandidates(flag)
	for _, path := range tried {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Paths{}, Config{}, err
		}
		c, err := cfg(real)
		if err != nil {
			return Paths{}, Config{}, err
		}
		base := filepath.Dir(real)
		abs := func(p string) string {
			if filepath.IsAbs(p) {
				return p
			}
			return filepath.Join(base, p)
		}
		return Paths{
			Config: real,
			Base:   base,
			Sheet:  abs(cmp.Or(c.Sheet, "data/studio-inputs.csv")),
			State:  abs(cmp.Or(c.StateDir, "state")),
		}, c, nil
	}
	return Paths{}, Config{}, fmt.Errorf("no %s found; looked in:\n  %s\nset --config <path> or STUDIO_MAP_CONFIG, or run `make install` to link ~/.config/studio-map/%s",
		configName, strings.Join(tried, "\n  "), configName)
}
