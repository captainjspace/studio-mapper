package mix

import (
	"fmt"
	"strings"

	"studio/engine/internal/config"
	"studio/engine/internal/presets"
	"studio/engine/internal/sheet"
)

// KnownStacks collects every stack (and sub-stack) used by any rig's sheet, plus the stem split targets.
func KnownStacks(cfg config.Config, p config.Paths) map[string]bool {
	known := map[string]bool{}
	for _, name := range cfg.RigNames() {
		rc, err := cfg.WithRig(name)
		if err != nil {
			continue
		}
		inputs, err := sheet.Load(p.Abs(rc.Sheet), rc)
		if err != nil {
			continue
		}
		for _, in := range sheet.ActiveInputs(inputs) {
			top, sub, _ := strings.Cut(in.Stack, "/")
			known[top] = true
			if sub != "" {
				known[sub] = true
			}
		}
	}
	for _, stack := range cfg.StemSplit {
		known[stack] = true
	}
	delete(known, "")
	return known
}

// RigGraph builds this rig's graph, aware of stacks other rigs use.
func RigGraph(cfg config.Config, inputs []sheet.Input, lib presets.Presets, known map[string]bool) Graph {
	g := Build(cfg, inputs, lib)
	g.Known = known
	g.PruneEmptyStacks()
	return g
}

// BuildRouting builds the mix graph from the rig's sheet (or Stem Splitter outputs) plus every problem found.
func BuildRouting(p config.Paths, cfg config.Config, stems bool) (Graph, presets.Presets, string, [][2]string, error) {
	inputs, err := sheet.Load(p.Sheet, cfg)
	if err != nil {
		return Graph{}, presets.Presets{}, "", nil, err
	}
	lib := presets.For(p, cfg)
	g := RigGraph(cfg, inputs, lib, KnownStacks(cfg, p))

	mode, problems := "session", [][2]string{}
	if stems {
		mode = "stems"
		known := g.Nodes
		stemTracks := sheet.StemInputs(cfg)
		g = Build(cfg, stemTracks, lib)
		g.Partial = true
		g.PruneEmptyStacks()
		for _, in := range stemTracks {
			if known[in.Stack] == nil {
				problems = append(problems, [2]string{in.Source, fmt.Sprintf("stack %q is not in the sheet or mix config", in.Stack)})
			}
		}
	}
	return g, lib, mode, append(problems, Validate(g, lib)...), nil
}
