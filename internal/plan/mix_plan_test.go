package plan

import (
	"strings"
	"testing"

	"studio/engine/internal/config"
	"studio/engine/internal/mix/mixtest"
	"studio/engine/internal/presets/presetstest"
)

func TestPlanMixListsPresetsToLoad(t *testing.T) {
	presets := presetstest.Library(t, "Bus/Smashed Guitars.cst", "Track/Kick In.cst", "Mix Bus/Mix Bus Comp.cst", "Output/Mastering.cst")
	p := Build(mixtest.Config(), mixtest.Inputs(), Live{Presets: presets})
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

	cfg := mixtest.Config()
	cfg.Mix["Mix_Bus"] = config.MixNode{Kind: "aux", Plugin: "I:O/UA2-1176.pst", Insert: &config.Insert{Rig: "home", Out: "3/4", In: "13/14"}}
	presets = presetstest.Library(t, "Plug-In Settings/I:O/UA2-1176.pst", "Output/Mastering.cst")
	var insertRows []Step
	for _, s := range Build(cfg, mixtest.Inputs(), Live{Presets: presets}).Steps {
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
