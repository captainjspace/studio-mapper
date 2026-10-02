// Command studio-map keeps the studio's MOTU devices and the Logic build sheet in line with the studio sheet.
package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"studio/engine/internal/config"
	"studio/engine/internal/export"
	"studio/engine/internal/mix"
	"studio/engine/internal/motu"
	"studio/engine/internal/plan"
	"studio/engine/internal/presets"
	"studio/engine/internal/server"
	"studio/engine/internal/sheet"
)

// command runs with the resolved paths and the selected rig's effective config.
type command func(p config.Paths, cfg config.Config, args []string) error

var commands = map[string]command{
	"plan":    runPlan,
	"apply":   runApply,
	"restore": runRestore,
	"routing": runRouting,
	"serve":   runServe,
	"inputs":  runInputs,
}

const usage = "usage: studio-map [--config <studio_config.json>] [--rig <name>] [--sheet <inputs.csv>] [plan [-v] | apply | routing [--stems] [--json] | inputs | serve [--addr :8080] | restore <state/snapshot.json>]"

func main() {
	configFlag, args := config.TakeFlag(os.Args[1:], "config")
	sheetFlag, args := config.TakeFlag(args, "sheet")
	rigFlag, args := config.TakeFlag(args, "rig")
	cmd := "plan"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	run, ok := commands[cmd]
	if !ok {
		fmt.Println(usage)
		os.Exit(2)
	}
	paths, cfg, err := config.Resolve(configFlag)
	if err != nil {
		fail(err)
	}
	rig, err := config.ChooseRig(cfg, rigFlag, cmd == "plan" || cmd == "apply")
	if err == nil {
		cfg, err = cfg.WithRig(rig)
	}
	if err != nil && cmd != "serve" {
		fail(err)
	}
	paths.Sheet = paths.Abs(cfg.Sheet)
	if sheetFlag != "" {
		paths.Sheet, _ = filepath.Abs(sheetFlag)
	}
	if err := run(paths, cfg, args); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Printf("❌ %v\n", err)
	os.Exit(1)
}

type session struct {
	inputs []sheet.Input
	client *http.Client
	live   plan.Live
	plan   plan.Plan
}

func newSession(p config.Paths, cfg config.Config) (*session, error) {
	inputs, err := sheet.Load(p.Sheet, cfg)
	if err != nil {
		return nil, err
	}
	s := &session{inputs: inputs, client: motu.NewClient()}
	s.live = plan.ReadLive(s.client, cfg, p)
	s.plan = plan.Build(cfg, inputs, s.live)
	return s, nil
}

func runPlan(p config.Paths, cfg config.Config, args []string) error {
	s, err := newSession(p, cfg)
	if err != nil {
		return err
	}
	plan.Print(s.plan, len(args) > 0 && args[0] == "-v")
	mix.PrintBlueprint(mix.RigGraph(cfg, s.inputs, s.live.Presets, s.live.Known))
	return nil
}

func runApply(p config.Paths, cfg config.Config, args []string) error {
	s, err := newSession(p, cfg)
	if err != nil {
		return err
	}
	plan.Print(s.plan, false)
	return plan.Apply(s.client, cfg, p, s.plan, s.live)
}

func runRestore(p config.Paths, cfg config.Config, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: studio-map restore <state/snapshot.json>")
	}
	return plan.Restore(motu.NewClient(), cfg, args[0])
}

// runInputs prints the flat input list as JSON.
func runInputs(p config.Paths, cfg config.Config, args []string) error {
	inputs, err := sheet.Load(p.Sheet, cfg)
	if err != nil {
		return err
	}
	return printJSON(export.Inputs(cfg.RigName, inputs, presets.For(p, cfg)))
}

// runRouting prints the Logic build sheet. It needs only the sheet, config and presets, not the rack.
func runRouting(p config.Paths, cfg config.Config, args []string) error {
	g, lib, mode, problems, err := mix.BuildRouting(p, cfg, slices.Contains(args, "--stems"))
	if err != nil {
		return err
	}
	if slices.Contains(args, "--json") {
		return printJSON(export.Routing(cfg.RigName, g, lib, mode, problems))
	}
	mix.PrintRouting(g, lib)
	for _, pr := range problems {
		fmt.Printf("  ⚠️  %s: %s\n", pr[0], pr[1])
	}
	mix.PrintBlueprint(g)
	return nil
}

// runServe serves the docs page and the routing JSON, regenerated from the sheet and config on every request.
func runServe(p config.Paths, cfg config.Config, args []string) error {
	addr, _ := config.TakeFlag(args, "addr")
	addr = cmp.Or(addr, ":8080")
	fmt.Printf("studio-map serving on %s (config %s)\n", addr, p.Config)
	return http.ListenAndServe(addr, server.New(p))
}

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
