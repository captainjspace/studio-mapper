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
	Known      map[string]bool // stacks used by any rig's sheet
}

func (p *Plan) add(s Status, scope, target, current, desired string) {
	p.Steps = append(p.Steps, Step{s, scope, target, current, desired})
}

// check records one managed setting: ok when it matches, otherwise a change queued for apply.
func (p *Plan) check(scope, target, current, desired string, same bool, dev, key, value string) {
	if same {
		p.add(StatusOK, scope, target, current, desired)
		return
	}
	p.add(StatusChange, scope, target, current, desired)
	if p.Writes[dev] == nil {
		p.Writes[dev] = map[string]string{}
	}
	p.Writes[dev][key] = value
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
	planOutputs(&p, cfg, live)
	planRoutes(&p, cfg, live)
	planStreams(&p, cfg, live)
	planHostNames(&p, cfg, active, live)
	planMix(&p, rigMixGraph(cfg, inputs, live.Presets, live.Known), active, live.Presets)
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
			p.check(dev, target, current, in.Label, current == in.Label, dev, key, in.Label)
		}
	}
}

// planOutputs manages output names declared in the rig's "outputs".
func planOutputs(p *Plan, cfg Config, live Live) {
	for _, dev := range sortedKeys(cfg.Outputs) {
		ds, ok := live.Datastores[dev]
		if !ok {
			continue // offline is already reported for the device
		}
		scope := dev + " outputs"
		for _, dest := range sortedKeys(cfg.Outputs[dev]) {
			want := SanitizeHardwareName(cfg.Outputs[dev][dest])
			key, err := ds.channelKey("obank", dest, "name")
			if err != nil {
				p.add(StatusWarn, scope, dest, "", err.Error())
				continue
			}
			current := ds.str(key)
			p.check(scope, dest, current, want, current == want, dev, key, want)
		}
	}
}

// planRoutes manages router sources declared in the rig's "routes" and flags undeclared computer loopbacks.
func planRoutes(p *Plan, cfg Config, live Live) {
	for _, dev := range sortedKeys(live.Datastores) {
		ds, routes := live.Datastores[dev], cfg.Routes[dev]
		scope := dev + " routes"
		for _, dest := range sortedKeys(routes) {
			key, err := ds.channelKey("obank", dest, "src")
			if err == nil {
				var want string
				if want, err = ds.sourceValue(routes[dest]); err == nil {
					current := ds.str(key)
					p.check(scope, dest+" ←", ds.sourceLabel(current), routes[dest], current == want, dev, key, want)
					continue
				}
			}
			p.add(StatusWarn, scope, dest+" ← "+routes[dest], "", err.Error())
		}
		if loops := computerLoopbacks(ds, routes); loops != "" {
			p.add(StatusWarn, scope, "To Computer "+loops, "From Computer", "loopback: feedback if Logic monitors these inputs while playing out the same channels")
		}
	}
}

// computerLoopbacks lists "To Computer N" channels fed from "From Computer", skipping declared routes.
func computerLoopbacks(ds Datastore, routes map[string]string) string {
	to, ok1 := ds.bankIdx("obank", "Computer")
	from, ok2 := ds.bankIdx("ibank", "Computer")
	if !ok1 || !ok2 {
		return ""
	}
	var chans []int
	for ch := 0; ; ch++ {
		src, ok := ds[fmt.Sprintf("ext/obank/%d/ch/%d/src", to, ch)]
		if !ok {
			break
		}
		_, declared := routes[fmt.Sprintf("Computer %d", ch+1)]
		if !declared && strings.HasPrefix(fmt.Sprint(src), fmt.Sprintf("%d:", from)) {
			chans = append(chans, ch+1)
		}
	}
	return compactRanges(chans)
}

// compactRanges renders 11,12,13,17 as "11–13, 17".
func compactRanges(ns []int) string {
	var parts []string
	for i := 0; i < len(ns); {
		j := i
		for j+1 < len(ns) && ns[j+1] == ns[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d–%d", ns[i], ns[j]))
		} else {
			parts = append(parts, strconv.Itoa(ns[i]))
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
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
