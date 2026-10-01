package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const (
	mixBus    = "Mix_Bus"
	stereoOut = "Stereo_Out"
)

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

// Insert is outboard gear patched in through Logic's I/O plugin on a given rig.
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

// Node is a resolved routing node: a sheet stack or sub-stack merged with its MixNode.
type Node struct {
	MixNode
	Name      string
	Parent    string
	Bus       int // input bus; 0 for the output
	Tracks    []Input
	FromSheet bool
}

type MixGraph struct {
	Nodes   map[string]*Node
	Order   []string // signal-flow order from the output
	Utility []Input  // inputs with no Stack: labeled on the hardware, not Logic tracks (e.g. Bluetooth to the PA)
	Partial bool     // built from Stem Splitter outputs: mix entries without tracks are expected
}

func buildMixGraph(cfg Config, inputs []Input, presets Presets) MixGraph {
	g := MixGraph{Nodes: map[string]*Node{}}
	node := func(name string) *Node {
		if n, ok := g.Nodes[name]; ok {
			return n
		}
		n := &Node{Name: name, MixNode: MixNode{Kind: "stack"}}
		g.Nodes[name] = n
		g.Order = append(g.Order, name)
		return n
	}

	for _, in := range activeInputs(inputs) {
		if in.HostIn == 0 && !in.Stem {
			continue
		}
		if in.Stack == "" {
			g.Utility = append(g.Utility, in)
			continue
		}
		top, sub, _ := strings.Cut(in.Stack, "/")
		leaf := node(top)
		leaf.FromSheet = true
		if sub != "" {
			leaf = node(sub)
			leaf.Parent, leaf.FromSheet = top, true
		}
		leaf.Tracks = append(leaf.Tracks, in)
	}

	for _, name := range sortedKeys(cfg.Mix) {
		n, m := node(name), cfg.Mix[name]
		kind := cmp.Or(m.Kind, n.Kind)
		n.MixNode = m
		n.Kind = kind
	}

	for _, n := range g.Nodes {
		if n.Preset == "" && n.Plugin == "" {
			n.Preset, _ = presets.find("Bus", n.Name)
		}
	}

	for _, name := range g.Order {
		n := g.Nodes[name]
		if n.Kind == "output" {
			continue
		}
		switch {
		case n.Output != "":
		case n.Parent != "":
			n.Output = n.Parent
		case name == mixBus:
			n.Output = stereoOut
		default:
			n.Output = mixBus
		}
	}
	g.numberBuses()
	return g
}

// pruneEmptyStacks drops stacks with no tracks and no children (e.g. per-player pans when mixing stem splits).
func (g *MixGraph) pruneEmptyStacks() {
	for changed := true; changed; {
		changed = false
		for _, name := range g.Order {
			n := g.Nodes[name]
			if n.Kind == "stack" && len(n.Tracks) == 0 && len(g.children(name)) == 0 {
				delete(g.Nodes, name)
				g.Order = slices.DeleteFunc(g.Order, func(s string) bool { return s == name })
				changed = true
				break
			}
		}
	}
	g.numberBuses()
}

// numberBuses walks the signal flow from the output so each group's buses are consecutive.
// Nodes that never reach the output are numbered last.
func (g *MixGraph) numberBuses() {
	var order []string
	var walk func(name string)
	walk = func(name string) {
		if slices.Contains(order, name) {
			return
		}
		order = append(order, name)
		for _, k := range g.children(name) {
			walk(k.Name)
		}
	}
	for _, name := range g.Order {
		if g.Nodes[name].Kind == "output" {
			walk(name)
		}
	}
	for _, name := range g.Order {
		walk(name)
	}
	bus := 0
	for _, name := range order {
		if n := g.Nodes[name]; n.Kind != "output" {
			bus++
			n.Bus = bus
		}
	}
	g.Order = order
}

// children returns the nodes that output into name, in graph order.
func (g MixGraph) children(name string) []*Node {
	var out []*Node
	for _, n := range g.Order {
		if g.Nodes[n].Output == name {
			out = append(out, g.Nodes[n])
		}
	}
	return out
}

// validateMix returns one problem per broken route or missing preset, keyed by node name.
func validateMix(g MixGraph, presets Presets) [][2]string {
	var problems [][2]string
	bad := func(name, format string, args ...any) {
		problems = append(problems, [2]string{name, fmt.Sprintf(format, args...)})
	}
	if _, ok := g.Nodes[stereoOut]; !ok {
		bad(stereoOut, "no %s node in mix config", stereoOut)
	}
	for _, name := range g.Order {
		n := g.Nodes[name]
		if n.Kind == "stack" && !n.FromSheet && !g.Partial {
			bad(name, "mix entry matches no Stack in the sheet")
		}
		if n.Kind != "output" {
			if _, ok := g.Nodes[n.Output]; !ok {
				bad(name, "output %q does not exist", n.Output)
			} else if !g.reachesOutput(name) {
				bad(name, "never reaches %s (cycle)", stereoOut)
			}
		}
		for _, s := range n.Sends {
			if _, ok := g.Nodes[s.To]; !ok {
				bad(name, "send target %q does not exist", s.To)
			}
		}
		if n.Preset != "" && !presets.exists(n.Preset) {
			bad(name, "preset %s not found", n.Preset)
		}
		if strings.HasSuffix(n.Plugin, ".pst") && !presets.pluginPresetExists(n.Plugin) {
			bad(name, "plugin preset %s not found", n.Plugin)
		}
	}
	return problems
}

func (g MixGraph) reachesOutput(name string) bool {
	var seen []string
	for n, ok := g.Nodes[name]; ok; n, ok = g.Nodes[n.Output] {
		if n.Kind == "output" {
			return true
		}
		if slices.Contains(seen, n.Name) {
			return false
		}
		seen = append(seen, n.Name)
	}
	return false
}
