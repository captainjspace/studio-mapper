// Package config holds studio_config.json: the shared mix model and each location's (rig's) device map.
package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// Config holds what is not per-channel; per-channel names and stacks come from each rig's sheet.
type Config struct {
	Rigs        map[string]Rig     `json:"rigs"`        // locations, each with its own device map
	DefaultRig  string             `json:"default_rig"` // used when nothing is detected or asked for
	PresetDir   string             `json:"preset_dir"`
	Mix         map[string]MixNode `json:"mix"`          // shared by every rig
	StemSplit   map[string]string  `json:"stem_split"`   // Stem Splitter output -> Stack
	StateDir    string             `json:"state_dir"`    // relative to this config file
	PresetIndex string             `json:"preset_index"` // relative to this config file; see `make presets-index`

	// Effective device map of the selected rig, filled in by WithRig.
	RigName    string                       `json:"-"`
	Devices    map[string]string            `json:"-"`
	Interfaces map[string]string            `json:"-"`
	HostDevice string                       `json:"-"`
	HostInputs []HostInput                  `json:"-"`
	Outputs    map[string]map[string]string `json:"-"`
	Routes     map[string]map[string]string `json:"-"`
	Sheet      string                       `json:"-"`
}

// Rig is one location's device map: what to query, how its inputs reach the host, and what to correct.
type Rig struct {
	Sheet      string                       `json:"sheet"`       // relative to the config file
	HostDevice string                       `json:"host_device"` // CoreAudio device Logic uses here
	Devices    map[string]string            `json:"devices"`     // name -> datastore URL ("" = no datastore, e.g. 10pre)
	Interfaces map[string]string            `json:"interfaces"`  // sheet "Interface" value -> device name
	HostInputs []HostInput                  `json:"host_inputs"`
	Outputs    map[string]map[string]string `json:"outputs,omitempty"` // device -> "Analog 1" -> name
	Routes     map[string]map[string]string `json:"routes,omitempty"`  // device -> destination "Analog 1" -> source "Mix Aux 5"
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

// MixNode is the config side of a routing node: everything the sheet can't say.
type MixNode struct {
	Kind   string  `json:"kind,omitempty"` // stack (default), aux, output
	Output string  `json:"output,omitempty"`
	Pan    *int    `json:"pan,omitempty"`
	Preset string  `json:"preset,omitempty"`
	Plugin string  `json:"plugin,omitempty"`
	Sends  []Send  `json:"sends,omitempty"`
	Insert *Insert `json:"insert,omitempty"`
}

// Insert is outboard gear on a given rig, patched via Logic's I/O plugin or the interface's own mixer.
type Insert struct {
	Rig string `json:"rig"`
	Out string `json:"out"`
	In  string `json:"in"`
}

func (i Insert) String() string {
	return fmt.Sprintf("%s: send %s, return %s; real-time bounce, or print the return", i.Rig, i.Out, i.In)
}

type Send struct {
	To    string  `json:"to"`
	Level float64 `json:"level"`
}

// Load reads and parses a config file.
func Load(path string) (Config, error) {
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

// RigNames lists the configured rigs in a stable order.
func (c Config) RigNames() []string { return slices.Sorted(maps.Keys(c.Rigs)) }

// WithRig returns the effective config for one rig: the shared parts plus that rig's device map.
func (c Config) WithRig(name string) (Config, error) {
	r, ok := c.Rigs[name]
	if !ok {
		return c, fmt.Errorf("no rig %q in config (have %s)", name, strings.Join(c.RigNames(), ", "))
	}
	c.RigName = name
	c.Devices, c.Interfaces, c.HostDevice, c.HostInputs = r.Devices, r.Interfaces, r.HostDevice, r.HostInputs
	c.Outputs, c.Routes = r.Outputs, r.Routes
	c.Sheet = cmp.Or(r.Sheet, "data/studio-inputs.csv")
	return c, nil
}

func (c Config) DefaultRigName() string {
	if c.DefaultRig != "" {
		return c.DefaultRig
	}
	if names := c.RigNames(); len(names) > 0 {
		return names[0]
	}
	return ""
}

// DeviceURL finds a device's URL in any rig (restore works wherever the snapshot came from).
func (c Config) DeviceURL(device string) string {
	for _, r := range c.Rigs {
		if url := r.Devices[device]; url != "" {
			return url
		}
	}
	return ""
}
