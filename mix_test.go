package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pan(v int) *int { return &v }

// testPresets builds a fake "Audio Music Apps" library; paths starting with "Plug-In Settings/" go to its sibling folder.
func testPresets(t *testing.T, files ...string) Presets {
	root := t.TempDir()
	cst := filepath.Join(root, "Channel Strip Settings")
	for _, f := range files {
		path := filepath.Join(cst, f)
		if strings.HasPrefix(f, "Plug-In Settings/") {
			path = filepath.Join(root, f)
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, nil, 0o644)
	}
	return loadPresets(cst)
}

var mixInputs = []Input{
	{HostIn: 1, Label: "Josh_Gtr_L", Stack: "Smashed_Guitars/joshgtr", Active: true},
	{HostIn: 2, Label: "John_Gtr_Mic", Stack: "Smashed_Guitars/johngtr", Active: true},
	{HostIn: 3, Label: "Kick_In", Stack: "drums/kick", Active: true},
	{HostIn: 4, Label: "Spare", Stack: "drums/kick", Active: false},
	{HostIn: 33, Label: "BT_L", Active: true},
}

func mixConfig() Config {
	return Config{Mix: map[string]MixNode{
		"joshgtr":    {Pan: pan(-64)},
		"drums":      {Sends: []Send{{To: "Drum_Verb", Level: -12}}},
		"Drum_Verb":  {Kind: "aux", Plugin: "Quantec Room Simulator"},
		"Mix_Bus":    {Kind: "aux", Preset: "Mix Bus/Mix Bus Comp.cst"},
		"Stereo_Out": {Kind: "output", Preset: "Output/Mastering.cst"},
	}}
}

func TestMixGraphDefaultsAndBuses(t *testing.T) {
	g := buildMixGraph(mixConfig(), mixInputs, testPresets(t, "Bus/Smashed Guitars.cst"))

	outputs := map[string]string{
		"joshgtr": "Smashed_Guitars", "johngtr": "Smashed_Guitars", "kick": "drums",
		"Smashed_Guitars": mixBus, "drums": mixBus, "Drum_Verb": mixBus, mixBus: stereoOut,
	}
	for name, want := range outputs {
		if got := g.Nodes[name].Output; got != want {
			t.Errorf("%s → %q, want %q", name, got, want)
		}
	}
	order := []string{mixBus, "Smashed_Guitars", "joshgtr", "johngtr", "drums", "kick", "Drum_Verb"}
	for i, name := range order {
		if g.Nodes[name].Bus != i+1 {
			t.Errorf("%s on Bus %d, want %d (order %v)", name, g.Nodes[name].Bus, i+1, g.Order)
		}
	}
	if g.Nodes[stereoOut].Bus != 0 {
		t.Error("output must not take a bus")
	}
	if len(g.Utility) != 1 || g.Utility[0].Label != "BT_L" {
		t.Errorf("unstacked input should be utility, got %+v", g.Utility)
	}
	if len(g.Nodes["kick"].Tracks) != 1 {
		t.Error("inactive rows must not be routed")
	}
	if got := g.Nodes["Smashed_Guitars"].Preset; got != "Bus/Smashed Guitars.cst" {
		t.Errorf("Smashed_Guitars preset = %q, want name-matched Bus/Smashed Guitars.cst", got)
	}
}

func TestValidateMix(t *testing.T) {
	cfg := mixConfig()
	cfg.Mix["drums"] = MixNode{Sends: []Send{{To: "Room_Verb"}}}
	cfg.Mix["Drum_Verb"] = MixNode{Kind: "aux", Plugin: "QRS", Output: "Loop_B"}
	cfg.Mix["FX"] = MixNode{Kind: "aux", Plugin: "Tape Delay/gone.pst"}
	cfg.Mix["Loop_B"] = MixNode{Kind: "aux", Plugin: "x", Output: "Drum_Verb"}
	cfg.Mix["Punchy_Bass"] = MixNode{Preset: "Bus/PunchyWarmBass.cst"}

	presets := testPresets(t, "Mix Bus/Mix Bus Comp.cst", "Plug-In Settings/Quantec Room Simulator/StudioDrums.pst")
	var got []string
	for _, p := range validateMix(buildMixGraph(cfg, mixInputs, presets), presets) {
		got = append(got, p[0]+": "+p[1])
	}
	for _, want := range []string{
		`drums: send target "Room_Verb" does not exist`,
		"Drum_Verb: never reaches Stereo_Out (cycle)",
		"Punchy_Bass: mix entry matches no Stack in the sheet",
		"Punchy_Bass: preset Bus/PunchyWarmBass.cst not found",
		"Stereo_Out: preset Output/Mastering.cst not found",
		"FX: plugin preset Tape Delay/gone.pst not found",
	} {
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("missing problem %q in:\n%s", want, strings.Join(got, "\n"))
		}
	}
}

func TestPlanMixListsPresetsToLoad(t *testing.T) {
	presets := testPresets(t, "Bus/Smashed Guitars.cst", "Track/Kick In.cst", "Mix Bus/Mix Bus Comp.cst", "Output/Mastering.cst")
	p := buildPlan(mixConfig(), mixInputs, Live{Presets: presets})
	got := map[string]Step{}
	for _, s := range p.Steps {
		if s.Scope == "Logic mix" {
			got[s.Target] = s
		}
	}
	for target, want := range map[string]Status{
		"stack Smashed_Guitars": StatusManual,
		"aux Mix_Bus":           StatusManual,
		"output Stereo_Out":     StatusManual,
		"track Kick_In":         StatusManual,
		"stack drums":           StatusWarn,
	} {
		if got[target].Status != want {
			t.Errorf("%s = %+v, want %s", target, got[target], want)
		}
	}
	if _, ok := got["aux Drum_Verb"]; ok {
		t.Error("a plugin-only aux needs no preset row")
	}

	cfg := mixConfig()
	cfg.Mix["Mix_Bus"] = MixNode{Kind: "aux", Plugin: "I:O/UA2-1176.pst", Insert: &Insert{Rig: "home", Out: "3/4", In: "13/14"}}
	presets = testPresets(t, "Plug-In Settings/I:O/UA2-1176.pst", "Output/Mastering.cst")
	var insertRows []Step
	for _, s := range buildPlan(cfg, mixInputs, Live{Presets: presets}).Steps {
		if s.Target == "aux Mix_Bus" {
			insertRows = append(insertRows, s)
		}
	}
	if len(insertRows) != 1 || insertRows[0].Status != StatusManual || !strings.Contains(insertRows[0].Desired, "real-time bounce") {
		t.Errorf("want one manual real-time-bounce row for the 1176 insert, got %+v", insertRows)
	}
	if _, ok := got["aux Drum_Verb"]; ok {
		t.Error("a plugin-only aux needs no preset row")
	}
}

func TestStemSplitRouting(t *testing.T) {
	cfg := mixConfig()
	cfg.StemSplit = map[string]string{"Drums": "drums", "Guitar": "Smashed_Guitars", "Vocals": "Screaming_Demons"}
	presets := testPresets(t, "Bus/Mix Bus Comp.cst", "Output/Mastering.cst")

	g := buildMixGraph(cfg, stemInputs(cfg), presets)
	g.Partial = true
	g.pruneEmptyStacks()
	if _, ok := g.Nodes["joshgtr"]; ok {
		t.Error("empty per-player stack should be pruned in stem mode")
	}
	for stack, label := range map[string]string{"drums": "Split_Drums", "Smashed_Guitars": "Split_Guitar", "Screaming_Demons": "Split_Vocals"} {
		n := g.Nodes[stack]
		if n == nil || len(n.Tracks) != 1 || n.Tracks[0].Label != label || n.Output != mixBus {
			t.Errorf("%s: want %s → Mix_Bus, got %+v", stack, label, n)
		}
	}
	for _, p := range validateMix(g, presets) {
		if strings.Contains(p[1], "matches no Stack") {
			t.Errorf("stem mode must not flag unused mix entries: %v", p)
		}
	}
	if got := stemInputs(cfg)[0].where(); got != "Stem" {
		t.Errorf("stem track source = %q, want Stem", got)
	}
}
