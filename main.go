package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// env is what every command reads: where the files are and the parsed config.
var env struct {
	paths Paths
	cfg   Config
}

// Config holds what is not per-channel; per-channel names and stacks come from each rig's sheet.
type Config struct {
	Rigs        map[string]Rig     `json:"rigs"`        // locations, each with its own device map
	DefaultRig  string             `json:"default_rig"` // used when nothing is detected or asked for
	PresetDir   string             `json:"preset_dir"`
	Mix         map[string]MixNode `json:"mix"`          // shared by every rig
	StemSplit   map[string]string  `json:"stem_split"`   // Stem Splitter output -> Stack
	StateDir    string             `json:"state_dir"`    // relative to this config file
	PresetIndex string             `json:"preset_index"` // relative to this config file; see `make presets-index`

	// Effective device map of the selected rig, filled in by withRig.
	RigName    string                       `json:"-"`
	Devices    map[string]string            `json:"-"`
	Interfaces map[string]string            `json:"-"`
	HostDevice string                       `json:"-"`
	HostInputs []HostInput                  `json:"-"`
	Outputs    map[string]map[string]string `json:"-"`
	Routes     map[string]map[string]string `json:"-"`
	Sheet      string                       `json:"-"`
}

// HostInput maps a block of device channels onto the host's Host In numbers.
type HostInput struct {
	Device    string `json:"device"`
	Bank      string `json:"bank"`
	Count     int    `json:"count"`
	HostStart int    `json:"host_start"`
	From      int    `json:"from,omitempty"`       // first bank channel of the block (default 1)
	AVBStream int    `json:"avb_stream,omitempty"` // first host AVB input stream carrying this block
}

var commands = map[string]func(args []string) error{
	"plan":    runPlan,
	"apply":   runApply,
	"restore": runRestore,
	"routing": runRouting,
	"serve":   runServe,
	"inputs":  runInputs,
}

func main() {
	configFlag, args := takeFlag(os.Args[1:], "config")
	sheetFlag, args := takeFlag(args, "sheet")
	rigFlag, args := takeFlag(args, "rig")
	cmd := "plan"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	run, ok := commands[cmd]
	if !ok {
		fmt.Println("usage: studio-map [--config <studio_config.json>] [--rig <name>] [--sheet <inputs.csv>] [plan [-v] | apply | routing [--stems] [--json] | inputs | serve [--addr :8080] | restore <state/snapshot.json>]")
		os.Exit(2)
	}
	paths, cfg, err := resolvePaths(configFlag, loadConfig)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	rig, err := chooseRig(cfg, rigFlag, cmd == "plan" || cmd == "apply")
	if err == nil {
		cfg, err = cfg.withRig(rig)
	}
	if err != nil && cmd != "serve" {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	paths.Sheet = paths.abs(cfg.Sheet)
	if sheetFlag != "" {
		paths.Sheet, _ = filepath.Abs(sheetFlag)
	}
	env.paths, env.cfg = paths, cfg
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
	live := Live{Datastores: map[string]Datastore{}, Presets: presetsFor(env.paths, cfg), Known: knownStacks(cfg, env.paths)}
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
	cfg := env.cfg
	inputs, err := LoadSheet(env.paths.Sheet, cfg)
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
	GenerateConsoleBlueprint(rigMixGraph(s.cfg, s.inputs, s.live.Presets, s.live.Known))
	return nil
}

// buildRouting builds the mix graph from the sheet (or Stem Splitter outputs) plus every problem found.
func buildRouting(p Paths, cfg Config, stems bool) (MixGraph, Presets, string, [][2]string, error) {
	inputs, err := LoadSheet(p.Sheet, cfg)
	if err != nil {
		return MixGraph{}, Presets{}, "", nil, err
	}
	presets := presetsFor(p, cfg)
	g := rigMixGraph(cfg, inputs, presets, knownStacks(cfg, p))

	mode, problems := "session", [][2]string{}
	if stems {
		mode = "stems"
		known := g.Nodes
		stemTracks := stemInputs(cfg)
		g = buildMixGraph(cfg, stemTracks, presets)
		g.Partial = true
		g.pruneEmptyStacks()
		for _, in := range stemTracks {
			if known[in.Stack] == nil {
				problems = append(problems, [2]string{in.Source, fmt.Sprintf("stack %q is not in the sheet or mix config", in.Stack)})
			}
		}
	}
	return g, presets, mode, append(problems, validateMix(g, presets)...), nil
}

// runInputs prints the flat input list as JSON.
func runInputs(args []string) error {
	inputs, err := LoadSheet(env.paths.Sheet, env.cfg)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(inputsDoc(env.cfg.RigName, inputs, presetsFor(env.paths, env.cfg)), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

// runRouting prints the Logic build sheet. It needs only the sheet, config and presets, not the rack.
func runRouting(args []string) error {
	g, presets, mode, problems, err := buildRouting(env.paths, env.cfg, slices.Contains(args, "--stems"))
	if err != nil {
		return err
	}
	if slices.Contains(args, "--json") {
		out, err := json.MarshalIndent(routingDoc(env.cfg.RigName, g, presets, mode, problems), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	printRouting(g, presets)
	for _, pr := range problems {
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
		snap, err := saveSnapshot(env.paths.State, dev, url, s.live.Datastores[dev])
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
		fmt.Printf("✅ %s: %d settings written and verified\n", dev, len(writes))
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
	url := cmp.Or(env.cfg.deviceURL(snap.Device), snap.URL)
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
	fmt.Printf("⏪ %s: restored %d settings from %s\n", snap.Device, len(diff), snap.Taken.Format(time.RFC3339))
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
