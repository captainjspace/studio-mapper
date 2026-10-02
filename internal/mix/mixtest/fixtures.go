// Package mixtest holds the small mix fixtures shared by the mix, plan and export tests.
package mixtest

import (
	"studio/engine/internal/config"
	"studio/engine/internal/sheet"
)

// Pan returns a pointer for config.MixNode.Pan.
func Pan(v int) *int { return &v }

// Inputs: two guitar sub-stacks, a kick sub-stack (one inactive row), and one unstacked utility input.
func Inputs() []sheet.Input {
	return []sheet.Input{
		{HostIn: 1, Label: "Josh_Gtr_L", Stack: "Smashed_Guitars/joshgtr", Active: true},
		{HostIn: 2, Label: "John_Gtr_Mic", Stack: "Smashed_Guitars/johngtr", Active: true},
		{HostIn: 3, Label: "Kick_In", Stack: "drums/kick", Active: true},
		{HostIn: 4, Label: "Spare", Stack: "drums/kick", Active: false},
		{HostIn: 33, Label: "BT_L", Active: true},
	}
}

// Config: a pan, a send, a reverb aux, the mix bus and the output.
func Config() config.Config {
	return config.Config{Mix: map[string]config.MixNode{
		"joshgtr":    {Pan: Pan(-64)},
		"drums":      {Sends: []config.Send{{To: "Drum_Verb", Level: -12}}},
		"Drum_Verb":  {Kind: "aux", Plugin: "Quantec Room Simulator"},
		"Mix_Bus":    {Kind: "aux", Preset: "Mix Bus/Mix Bus Comp.cst"},
		"Stereo_Out": {Kind: "output", Preset: "Output/Mastering.cst"},
	}}
}
