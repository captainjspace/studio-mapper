package mix

import (
	"strings"
	"testing"

	"studio/engine/internal/config"
	"studio/engine/internal/mix/mixtest"
	"studio/engine/internal/presets/presetstest"
	"studio/engine/internal/sheet"
)

func TestMixGraphDefaultsAndBuses(t *testing.T) {
	g := Build(mixtest.Config(), mixtest.Inputs(), presetstest.Library(t, "Bus/Smashed Guitars.cst"))

	outputs := map[string]string{
		"joshgtr": "Smashed_Guitars", "johngtr": "Smashed_Guitars", "kick": "drums",
		"Smashed_Guitars": MixBus, "drums": MixBus, "Drum_Verb": MixBus, MixBus: StereoOut,
	}
	for name, want := range outputs {
		if got := g.Nodes[name].Output; got != want {
			t.Errorf("%s → %q, want %q", name, got, want)
		}
	}
	order := []string{MixBus, "Smashed_Guitars", "joshgtr", "johngtr", "drums", "kick", "Drum_Verb"}
	for i, name := range order {
		if g.Nodes[name].Bus != i+1 {
			t.Errorf("%s on Bus %d, want %d (order %v)", name, g.Nodes[name].Bus, i+1, g.Order)
		}
	}
	if g.Nodes[StereoOut].Bus != 0 {
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
	cfg := mixtest.Config()
	cfg.Mix["drums"] = config.MixNode{Sends: []config.Send{{To: "Room_Verb"}}}
	cfg.Mix["Drum_Verb"] = config.MixNode{Kind: "aux", Plugin: "QRS", Output: "Loop_B"}
	cfg.Mix["FX"] = config.MixNode{Kind: "aux", Plugin: "Tape Delay/gone.pst"}
	cfg.Mix["Loop_B"] = config.MixNode{Kind: "aux", Plugin: "x", Output: "Drum_Verb"}
	cfg.Mix["Punchy_Bass"] = config.MixNode{Preset: "Bus/PunchyWarmBass.cst"}

	presets := presetstest.Library(t, "Mix Bus/Mix Bus Comp.cst", "Plug-In Settings/Quantec Room Simulator/StudioDrums.pst")
	var got []string
	for _, p := range Validate(Build(cfg, mixtest.Inputs(), presets), presets) {
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

func TestStemSplitRouting(t *testing.T) {
	cfg := mixtest.Config()
	cfg.StemSplit = map[string]string{"Drums": "drums", "Guitar": "Smashed_Guitars", "Vocals": "Screaming_Demons"}
	presets := presetstest.Library(t, "Bus/Mix Bus Comp.cst", "Output/Mastering.cst")

	g := Build(cfg, sheet.StemInputs(cfg), presets)
	g.Partial = true
	g.PruneEmptyStacks()
	if _, ok := g.Nodes["joshgtr"]; ok {
		t.Error("empty per-player stack should be pruned in stem mode")
	}
	for stack, label := range map[string]string{"drums": "Split_Drums", "Smashed_Guitars": "Split_Guitar", "Screaming_Demons": "Split_Vocals"} {
		n := g.Nodes[stack]
		if n == nil || len(n.Tracks) != 1 || n.Tracks[0].Label != label || n.Output != MixBus {
			t.Errorf("%s: want %s → Mix_Bus, got %+v", stack, label, n)
		}
	}
	for _, p := range Validate(g, presets) {
		if strings.Contains(p[1], "matches no Stack") {
			t.Errorf("stem mode must not flag unused mix entries: %v", p)
		}
	}
	if got := sheet.StemInputs(cfg)[0].Where(); got != "Stem" {
		t.Errorf("stem track source = %q, want Stem", got)
	}
}
