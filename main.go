package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	configPath = "studio_config.json"
	sheetPath  = "studio-inputs.csv"
)

// Config holds what is not per-channel; per-channel names and stacks come from the sheet.
type Config struct {
	Devices    map[string]string  `json:"devices"`
	Interfaces map[string]string  `json:"interfaces"`
	HostDevice string             `json:"host_device"`
	HostInputs []HostInput        `json:"host_inputs"`
	PresetDir  string             `json:"preset_dir"`
	Mix        map[string]MixNode `json:"mix"`
	StemSplit  map[string]string  `json:"stem_split"` // Stem Splitter output -> Stack
}

// HostInput maps a block of device channels onto the host's Host In numbers.
type HostInput struct {
	Device    string `json:"device"`
	Bank      string `json:"bank"`
	Count     int    `json:"count"`
	HostStart int    `json:"host_start"`
	AVBStream int    `json:"avb_stream,omitempty"` // first host AVB input stream carrying this block
}

var commands = map[string]func(args []string) error{
	"plan":    runPlan,
	"apply":   runApply,
	"restore": runRestore,
	"routing": runRouting,
}

func main() {
	cmd, args := "plan", os.Args[1:]
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	run, ok := commands[cmd]
	if !ok {
		fmt.Println("usage: studio-map [plan [-v] | apply | routing [--stems] | restore <state/snapshot.json>]")
		os.Exit(2)
	}
	if err := run(args); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

// newClient avoids keep-alive: MOTU devices send a stray CRLF after 204 replies.
func newClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func readLive(client *http.Client, cfg Config) Live {
	live := Live{Datastores: map[string]Datastore{}, Presets: loadPresets(cmp.Or(cfg.PresetDir, defaultPresetDir))}
	for _, dev := range sortedKeys(cfg.Devices) {
		url := cfg.Devices[dev]
		if url == "" {
			continue
		}
		ds, err := fetchDatastore(client, url)
		if err != nil {
			fmt.Printf("⚠️  %s unreachable (%v)\n", dev, err)
			continue
		}
		fmt.Printf("📡 %s: read %d datastore keys\n", dev, len(ds))
		live.Datastores[dev] = ds
	}
	if names, err := hostInNames(cfg.HostDevice); err == nil {
		fmt.Printf("🎧 %s: read %d CoreAudio input names\n", cfg.HostDevice, len(names))
		live.HostNames = names
	}
	return live
}

type session struct {
	cfg    Config
	inputs []Input
	client *http.Client
	live   Live
	plan   Plan
}

func newSession() (*session, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	inputs, err := LoadSheet(sheetPath, cfg)
	if err != nil {
		return nil, err
	}
	s := &session{cfg: cfg, inputs: inputs, client: newClient()}
	s.live = readLive(s.client, cfg)
	s.plan = buildPlan(cfg, inputs, s.live)
	return s, nil
}

func runPlan(args []string) error {
	s, err := newSession()
	if err != nil {
		return err
	}
	printPlan(s.plan, len(args) > 0 && args[0] == "-v")
	GenerateConsoleBlueprint(buildMixGraph(s.cfg, s.inputs, s.live.Presets))
	return nil
}

// runRouting prints the Logic build sheet. It needs only the sheet, config and presets, not the rack.
func runRouting(args []string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	inputs, err := LoadSheet(sheetPath, cfg)
	if err != nil {
		return err
	}
	presets := loadPresets(cmp.Or(cfg.PresetDir, defaultPresetDir))
	g := buildMixGraph(cfg, inputs, presets)
	if len(args) > 0 && args[0] == "--stems" {
		known := g.Nodes
		stems := stemInputs(cfg)
		g = buildMixGraph(cfg, stems, presets)
		g.Partial = true
		g.pruneEmptyStacks()
		for _, in := range stems {
			if known[in.Stack] == nil {
				fmt.Printf("  ⚠️  %s: stack %q is not in the sheet or mix config\n", in.Source, in.Stack)
			}
		}
	}
	printRouting(g, presets)
	for _, pr := range validateMix(g, presets) {
		fmt.Printf("  ⚠️  %s: %s\n", pr[0], pr[1])
	}
	GenerateConsoleBlueprint(g)
	return nil
}

func runApply(args []string) error {
	s, err := newSession()
	if err != nil {
		return err
	}
	printPlan(s.plan, false)

	for _, dev := range sortedKeys(s.plan.Writes) {
		writes := s.plan.Writes[dev]
		url := s.cfg.Devices[dev]
		snap, err := saveSnapshot(stateDir, dev, url, s.live.Datastores[dev])
		if err != nil {
			return fmt.Errorf("snapshot %s: %w", dev, err)
		}
		fmt.Printf("\n💾 %s snapshot: %s\n", dev, snap)
		if err := writeKeys(s.client, url, writes); err != nil {
			return fmt.Errorf("write %s: %w", dev, err)
		}
		if err := verifyWrites(s.client, url, writes); err != nil {
			return fmt.Errorf("verify %s: %w", dev, err)
		}
		fmt.Printf("✅ %s: %d input names written and verified\n", dev, len(writes))
	}

	if n := s.plan.count(StatusManual); n > 0 {
		fmt.Printf("\n✋ %d items to do by hand in CueMix/Logic (see ✋ rows above).\n", n)
	}
	return nil
}

func runRestore(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: studio-map restore <state/snapshot.json>")
	}
	snap, err := loadSnapshot(args[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	url := cfg.Devices[snap.Device]
	if url == "" {
		url = snap.URL
	}
	client := newClient()
	ds, err := fetchDatastore(client, url)
	if err != nil {
		return fmt.Errorf("%s unreachable: %w", snap.Device, err)
	}
	diff := changedNames(ds, snap.Names)
	if err := writeKeys(client, url, diff); err != nil {
		return err
	}
	if err := verifyWrites(client, url, diff); err != nil {
		return err
	}
	fmt.Printf("⏪ %s: restored %d input names from %s\n", snap.Device, len(diff), snap.Taken.Format(time.RFC3339))
	return nil
}

func verifyWrites(client *http.Client, url string, writes map[string]string) error {
	ds, err := fetchDatastore(client, url)
	if err != nil {
		return err
	}
	if diff := changedNames(ds, writes); len(diff) > 0 {
		return fmt.Errorf("%d names did not stick: %v", len(diff), sortedKeys(diff))
	}
	return nil
}

// SanitizeHardwareName cleans strings to prevent MOTU hardware internal errors
func SanitizeHardwareName(rawName string) string {
	spaced := strings.ReplaceAll(rawName, " ", "_")

	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			return r
		}
		return -1
	}, spaced)

	if len(sanitized) > 31 {
		return sanitized[:31]
	}
	return sanitized
}
