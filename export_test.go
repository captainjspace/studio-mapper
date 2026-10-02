package main

import (
	"encoding/json"
	"testing"
)

// roundTrip marshals the doc and decodes it back, as the band app would read it.
func roundTrip(t *testing.T, doc RoutingDoc) RoutingDoc {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var back RoutingDoc
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	return back
}

func findNode(n *NodeJSON, name string) *NodeJSON {
	if n == nil || n.Name == name {
		return n
	}
	for _, k := range n.Children {
		if f := findNode(k, name); f != nil {
			return f
		}
	}
	return nil
}

func TestRoutingDocSession(t *testing.T) {
	inputs := append([]Input{}, mixInputs...)
	inputs[2].Musician, inputs[2].SoundSource, inputs[2].Mic, inputs[2].Preamp, inputs[2].Source = "Erick", "Kick In", "Audix D6", "API 3124", "Motu 16A A3"
	presets := testPresets(t, "Track/Kick In.cst")
	g := buildMixGraph(mixConfig(), inputs, presets)

	doc := roundTrip(t, routingDoc(g, presets, "session", validateMix(g, presets)))

	if doc.Version != 1 || doc.Mode != "session" {
		t.Errorf("version/mode = %d/%s", doc.Version, doc.Mode)
	}
	if doc.Root == nil || doc.Root.Name != stereoOut || len(doc.Root.Children) != 1 || doc.Root.Children[0].Name != mixBus {
		t.Fatalf("want Stereo_Out → Mix_Bus at the root, got %+v", doc.Root)
	}
	drums := findNode(doc.Root, "drums")
	if drums == nil || len(drums.Sends) != 1 || drums.Sends[0].To != "Drum_Verb" {
		t.Errorf("drums should send to Drum_Verb, got %+v", drums)
	}
	kick := findNode(doc.Root, "kick")
	if kick == nil || len(kick.Tracks) != 1 {
		t.Fatalf("kick stack should hold one track, got %+v", kick)
	}
	want := TrackJSON{Label: "Kick_In", Musician: "Erick", Source: "Kick In", Mic: "Audix D6", Preamp: "API 3124",
		Interface: "Motu 16A A3", HostIn: 3, ChannelPreset: "Track/Kick In.cst"}
	if kick.Tracks[0] != want {
		t.Errorf("Kick_In = %+v, want %+v", kick.Tracks[0], want)
	}
	if josh := findNode(doc.Root, "joshgtr"); josh == nil || josh.Pan == nil || *josh.Pan != -64 {
		t.Errorf("joshgtr pan should survive the round trip, got %+v", josh)
	}
	if len(doc.NotRecorded) != 1 || doc.NotRecorded[0].Label != "BT_L" {
		t.Errorf("utility input belongs in notRecorded, got %+v", doc.NotRecorded)
	}
	if len(doc.Warnings) == 0 {
		t.Error("missing presets should surface as warnings")
	}
}

func TestRoutingDocStems(t *testing.T) {
	cfg := mixConfig()
	cfg.StemSplit = map[string]string{"Drums": "drums", "Vocals": "Screaming_Demons"}
	presets := testPresets(t)
	g := buildMixGraph(cfg, stemInputs(cfg), presets)
	g.Partial = true
	g.pruneEmptyStacks()

	doc := roundTrip(t, routingDoc(g, presets, "stems", nil))
	drums := findNode(doc.Root, "drums")
	if drums == nil || len(drums.Tracks) != 1 {
		t.Fatalf("drums should hold the drum stem, got %+v", drums)
	}
	if tr := drums.Tracks[0]; !tr.Stem || tr.HostIn != 0 || tr.Interface != "" || tr.Source != "Drums" {
		t.Errorf("stem track = %+v, want stem with source Drums and no patch point", tr)
	}
	if doc.Warnings == nil {
		t.Error("warnings should be an empty list, not null, for the app")
	}
}
