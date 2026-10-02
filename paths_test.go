package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, configName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolvePathsFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `{"rigs": {"oakland": {"sheet": "data/inputs.csv"}}}`)
	sub := filepath.Join(root, "templates", "deep")
	_ = os.MkdirAll(sub, 0o755)
	t.Chdir(sub)
	t.Setenv("STUDIO_MAP_CONFIG", "")

	p, cfg, err := resolvePaths("", loadConfig)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := cfg.withRig(cfg.defaultRig())
	if err != nil {
		t.Fatal(err)
	}
	realRoot, _ := filepath.EvalSymlinks(root)
	if p.abs(rc.Sheet) != filepath.Join(realRoot, "data/inputs.csv") || p.State != filepath.Join(realRoot, "state") {
		t.Errorf("paths should resolve against the config's folder, got %+v", p)
	}
}

func TestResolvePathsFlagEnvAndSymlink(t *testing.T) {
	repo := t.TempDir()
	cfg := writeConfig(t, repo, `{}`)
	link := filepath.Join(t.TempDir(), configName)
	if err := os.Symlink(cfg, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	t.Setenv("STUDIO_MAP_CONFIG", link)
	p, _, err := resolvePaths("", loadConfig)
	if err != nil {
		t.Fatal(err)
	}
	realRepo, _ := filepath.EvalSymlinks(repo)
	if p.Base != realRepo || p.abs("data/studio-inputs.csv") != filepath.Join(realRepo, "data/studio-inputs.csv") {
		t.Errorf("a linked config must resolve paths in the real repo, got %+v", p)
	}

	if _, _, err := resolvePaths(filepath.Join(repo, "nope.json"), loadConfig); err == nil || !strings.Contains(err.Error(), "nope.json") {
		t.Errorf("--config pointing nowhere should say where it looked, got %v", err)
	}
}

func TestTakeFlag(t *testing.T) {
	v, rest := takeFlag([]string{"--stems", "--sheet", "x.csv", "--json"}, "sheet")
	if v != "x.csv" || strings.Join(rest, " ") != "--stems --json" {
		t.Errorf("got %q %v", v, rest)
	}
	if v, rest := takeFlag([]string{"-v"}, "config"); v != "" || len(rest) != 1 {
		t.Errorf("absent flag should leave args alone, got %q %v", v, rest)
	}
}
