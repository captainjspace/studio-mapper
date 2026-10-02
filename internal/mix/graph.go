// Package mix models the Logic signal flow: tracks → stacks → buses → Mix_Bus → Stereo_Out.
package mix

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"studio/engine/internal/config"
	"studio/engine/internal/presets"
	"studio/engine/internal/sheet"
)

const (
	MixBus    = "Mix_Bus"
	StereoOut = "Stereo_Out"
)

// Node is a resolved routing node: a sheet stack or sub-stack merged with its config.MixNode.
type Node struct {
	config.MixNode
	Name      string
	Parent    string
	Bus       int // input bus; 0 for the output
	Tracks    []sheet.Input
	FromSheet bool
}

type Graph struct {
	Nodes   map[string]*Node
	Order   []string        // signal-flow order from the output
	Utility []sheet.Input   // inputs with no Stack: labeled on the hardware, not Logic tracks (e.g. Bluetooth to the PA)
	Partial bool            // built from Stem Splitter outputs: mix entries without tracks are expected
	Known   map[string]bool // stacks used by any rig's sheet; empty here but used elsewhere is fine
}

func Build(cfg config.Config, inputs []sheet.Input, lib presets.Presets) Graph {
	g := Graph{Nodes: map[string]*Node{}}
	node := func(name string) *Node {
		if n, ok := g.Nodes[name]; ok {
			return n
		}
		n := &Node{Name: name, MixNode: config.MixNode{Kind: "stack"}}
		g.Nodes[name] = n
		g.Order = append(g.Order, name)
		return n
	}

	for _, in := range sheet.ActiveInputs(inputs) {
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

	for _, name := range slices.Sorted(maps.Keys(cfg.Mix)) {
		n, m := node(name), cfg.Mix[name]
		kind := cmp.Or(m.Kind, n.Kind)
		n.MixNode = m
		n.Kind = kind
	}

	for _, n := range g.Nodes {
		if n.Preset == "" && n.Plugin == "" {
			n.Preset, _ = lib.Find("Bus", n.Name)
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
		case name == MixBus:
			n.Output = StereoOut
		default:
			n.Output = MixBus
		}
	}
	g.numberBuses()
	return g
}

// PruneEmptyStacks drops stacks with no tracks and no children that are expected to be empty here:
// any in stem mode (per-player pans), otherwise those another rig's sheet uses. True orphans stay and get warned.
func (g *Graph) PruneEmptyStacks() {
	for changed := true; changed; {
		changed = false
		for _, name := range g.Order {
			n := g.Nodes[name]
			if n.Kind == "stack" && len(n.Tracks) == 0 && len(g.Children(name)) == 0 && (g.Partial || g.Known[name]) {
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
func (g *Graph) numberBuses() {
	var order []string
	var walk func(name string)
	walk = func(name string) {
		if slices.Contains(order, name) {
			return
		}
		order = append(order, name)
		for _, k := range g.Children(name) {
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

// Children returns the nodes that output into name, in graph order.
func (g Graph) Children(name string) []*Node {
	var out []*Node
	for _, n := range g.Order {
		if g.Nodes[n].Output == name {
			out = append(out, g.Nodes[n])
		}
	}
	return out
}

// Validate returns one problem per broken route or missing preset, keyed by node name.
func Validate(g Graph, lib presets.Presets) [][2]string {
	var problems [][2]string
	bad := func(name, format string, args ...any) {
		problems = append(problems, [2]string{name, fmt.Sprintf(format, args...)})
	}
	if _, ok := g.Nodes[StereoOut]; !ok {
		bad(StereoOut, "no %s node in mix config", StereoOut)
	}
	for _, name := range g.Order {
		n := g.Nodes[name]
		if n.Kind == "stack" && !n.FromSheet && !g.Partial && !g.Known[name] {
			bad(name, "mix entry matches no Stack in the sheet")
		}
		if n.Kind != "output" {
			if _, ok := g.Nodes[n.Output]; !ok {
				bad(name, "output %q does not exist", n.Output)
			} else if !g.reachesOutput(name) {
				bad(name, "never reaches %s (cycle)", StereoOut)
			}
		}
		for _, s := range n.Sends {
			if _, ok := g.Nodes[s.To]; !ok {
				bad(name, "send target %q does not exist", s.To)
			}
		}
		if n.Preset != "" && !lib.Exists(n.Preset) {
			bad(name, "preset %s not found", n.Preset)
		}
		if strings.HasSuffix(n.Plugin, ".pst") && !lib.PluginPresetExists(n.Plugin) {
			bad(name, "plugin preset %s not found", n.Plugin)
		}
	}
	return problems
}

func (g Graph) reachesOutput(name string) bool {
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
