package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Status string

const (
	StatusOK     Status = "ok"
	StatusChange Status = "change"
	StatusManual Status = "manual"
	StatusWarn   Status = "warn"
)

var statusIcon = map[Status]string{
	StatusOK:     "✅",
	StatusChange: "✏️ ",
	StatusManual: "✋",
	StatusWarn:   "⚠️ ",
}

type Step struct {
	Status  Status
	Scope   string
	Target  string
	Current string
	Desired string
}

type Plan struct {
	Steps  []Step
	Writes map[string]map[string]string // device -> datastore key -> name
}

// Live is everything read from the rack before planning.
type Live struct {
	Datastores map[string]Datastore // reachable datastore devices only
	HostNames  []string             // CoreAudio input names of the host device; nil if unreadable
	Presets    Presets
}

func (p *Plan) add(s Status, scope, target, current, desired string) {
	p.Steps = append(p.Steps, Step{s, scope, target, current, desired})
}

func (p Plan) count(s Status) int {
	n := 0
	for _, st := range p.Steps {
		if st.Status == s {
			n++
		}
	}
	return n
}

func activeInputs(inputs []Input) []Input {
	var out []Input
	for _, in := range inputs {
		if in.Active {
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].HostIn < out[j].HostIn })
	return out
}

func buildPlan(cfg Config, inputs []Input, live Live) Plan {
	p := Plan{Writes: map[string]map[string]string{}}
	active := activeInputs(inputs)

	planValidation(&p, cfg, active)
	planDeviceNames(&p, cfg, active, live)
	planStreams(&p, cfg, live)
	planHostNames(&p, cfg, active, live)
	planMix(&p, buildMixGraph(cfg, inputs, live.Presets), active, live.Presets)
	return p
}

func planValidation(p *Plan, cfg Config, active []Input) {
	seen := map[int]Input{}
	for _, in := range active {
		if in.HostIn == 0 {
			p.add(StatusWarn, "sheet", fmt.Sprintf("row %d %s", in.Row, in.Source), "", in.Label+": not routed to the host")
			continue
		}
		if prev, dup := seen[in.HostIn]; dup {
			p.add(StatusWarn, "sheet", fmt.Sprintf("row %d Host In %d", in.Row, in.HostIn), prev.Label, in.Label+": duplicate Host In")
		}
		seen[in.HostIn] = in
	}
}

func planDeviceNames(p *Plan, cfg Config, active []Input, live Live) {
	for _, dev := range sortedKeys(cfg.Devices) {
		url := cfg.Devices[dev]
		if url == "" {
			continue
		}
		ds, ok := live.Datastores[dev]
		if !ok {
			p.add(StatusWarn, dev, url, "", "offline: skipped")
			continue
		}
		for _, in := range active {
			if in.Device != dev || in.HostIn == 0 {
				continue
			}
			target := fmt.Sprintf("%s %d (Host In %d)", in.Bank, in.Ch, in.HostIn)
			bank, ok := ds.bankIndex(in.Bank)
			if !ok {
				p.add(StatusWarn, dev, target, "", "no input bank named "+in.Bank)
				continue
			}
			key := inputNamePath(bank, in.Ch)
			current := ds.str(key)
			if current == in.Label {
				p.add(StatusOK, dev, target, current, in.Label)
				continue
			}
			p.add(StatusChange, dev, target, current, in.Label)
			if p.Writes[dev] == nil {
				p.Writes[dev] = map[string]string{}
			}
			p.Writes[dev][key] = in.Label
		}
	}
}

// planStreams checks that the host's AVB input streams listen to the talkers host_inputs expects.
func planStreams(p *Plan, cfg Config, live Live) {
	scope := cfg.HostDevice + " streams"
	for _, h := range cfg.HostInputs {
		if h.AVBStream == 0 {
			continue
		}
		talker, ok := live.Datastores[h.Device]
		if !ok {
			continue
		}
		listener, found := talker.avbEntity(cfg.HostDevice)
		if !found {
			p.add(StatusWarn, scope, cfg.HostDevice, "", "not visible on the AVB network from "+h.Device)
			continue
		}
		uid := talker.str("uid")
		for i := 0; i < (h.Count+7)/8; i++ {
			stream := h.AVBStream + i
			want := uid + ":" + strconv.Itoa(i)
			current := talker.streamTalker(listener, stream)
			status := StatusOK
			if current != want {
				status = StatusManual
			}
			p.add(status, scope, "Input Stream "+strconv.Itoa(stream),
				friendlyTalker(current, live), friendlyTalker(want, live))
		}
	}
}

func planHostNames(p *Plan, cfg Config, active []Input, live Live) {
	scope := cfg.HostDevice + " Host In"
	if live.HostNames == nil {
		p.add(StatusWarn, scope, cfg.HostDevice, "", "CoreAudio names unreadable: is the "+cfg.HostDevice+" connected?")
		return
	}
	for _, in := range active {
		if in.HostIn == 0 || in.HostIn > len(live.HostNames) {
			continue
		}
		current := live.HostNames[in.HostIn-1]
		status := StatusOK
		if current != in.Label {
			status = StatusManual
		}
		p.add(status, scope, "Host In "+strconv.Itoa(in.HostIn), current, in.Label)
	}
}

// planMix reports broken routes and lists the channel strip settings to load in Logic.
func planMix(p *Plan, g MixGraph, active []Input, presets Presets) {
	const scope = "Logic mix"
	problems := map[string]bool{}
	for _, pr := range validateMix(g, presets) {
		p.add(StatusWarn, scope, pr[0], "", pr[1])
		problems[pr[0]] = true
	}
	for _, name := range g.Order {
		n := g.Nodes[name]
		switch {
		case problems[name]:
		case n.Preset != "":
			p.add(StatusManual, scope, n.Kind+" "+name, "", n.Preset)
		case n.Plugin == "" && n.Parent == "":
			p.add(StatusWarn, scope, n.Kind+" "+name, "", "no .cst found: save one or set mix."+name+".preset")
		}
		if n.Insert != nil {
			p.add(StatusManual, scope, n.Kind+" "+name, n.Plugin, n.Insert.String())
		}
	}
	for _, in := range active {
		if rel, ok := presets.find("Track", in.Label); ok {
			p.add(StatusManual, scope, "track "+in.Label, "", rel)
		}
	}
}

func friendlyTalker(talker string, live Live) string {
	uid, idx, _ := strings.Cut(talker, ":")
	if strings.Trim(uid, "0") == "" {
		return "(not connected)"
	}
	for dev, ds := range live.Datastores {
		if ds.str("uid") == uid {
			n, _ := strconv.Atoi(idx)
			return fmt.Sprintf("%s stream %d", dev, n+1)
		}
	}
	return talker
}

func printPlan(p Plan, verbose bool) {
	scope := ""
	for _, s := range p.Steps {
		if s.Status == StatusOK && !verbose {
			continue
		}
		if s.Scope != scope {
			scope = s.Scope
			fmt.Printf("\n── %s\n", scope)
		}
		fmt.Printf("  %s %-28s %-24q → %q\n", statusIcon[s.Status], s.Target, s.Current, s.Desired)
	}
	fmt.Printf("\nPlan: %d ok, %d to change, %d manual (CueMix/Logic), %d warnings\n",
		p.count(StatusOK), p.count(StatusChange), p.count(StatusManual), p.count(StatusWarn))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
