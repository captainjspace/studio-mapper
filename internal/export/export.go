// Package export renders the routing tree and input list as the versioned JSON the docs page and band app read.
package export

import (
	"time"

	"studio/engine/internal/config"
	"studio/engine/internal/mix"
	"studio/engine/internal/presets"
	"studio/engine/internal/sheet"
)

// Version is bumped on breaking changes to the JSON the band app reads.
const Version = 1

type RoutingDoc struct {
	Version     int         `json:"version"`
	Generated   time.Time   `json:"generated"`
	Rig         string      `json:"rig,omitempty"`
	Mode        string      `json:"mode"` // "session" or "stems"
	Root        *NodeJSON   `json:"root,omitempty"`
	NotRecorded []TrackJSON `json:"notRecorded,omitempty"`
	Warnings    []Warning   `json:"warnings"`
}

type NodeJSON struct {
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Bus      int            `json:"bus,omitempty"`
	Pan      *int           `json:"pan,omitempty"`
	Preset   string         `json:"preset,omitempty"`
	Plugin   string         `json:"plugin,omitempty"`
	Insert   *config.Insert `json:"insert,omitempty"`
	Sends    []config.Send  `json:"sends,omitempty"`
	Children []*NodeJSON    `json:"children,omitempty"`
	Tracks   []TrackJSON    `json:"tracks,omitempty"`
}

type TrackJSON struct {
	Label         string `json:"label"`
	Musician      string `json:"musician,omitempty"`
	Source        string `json:"source,omitempty"`
	Mic           string `json:"mic,omitempty"`
	Preamp        string `json:"preamp,omitempty"`
	Interface     string `json:"interface,omitempty"`
	HostIn        int    `json:"hostIn,omitempty"`
	Stem          bool   `json:"stem,omitempty"`
	ChannelPreset string `json:"channelPreset,omitempty"`
}

type Warning struct {
	Node    string `json:"node"`
	Message string `json:"message"`
}

// Routing renders the mix graph as the nested tree the band app displays.
func Routing(rig string, g mix.Graph, lib presets.Presets, mode string, problems [][2]string) RoutingDoc {
	doc := RoutingDoc{Version: Version, Generated: time.Now(), Rig: rig, Mode: mode, Warnings: []Warning{}}
	if out, ok := g.Nodes[mix.StereoOut]; ok {
		doc.Root = nodeJSON(g, out, lib)
	}
	for _, in := range g.Utility {
		doc.NotRecorded = append(doc.NotRecorded, trackJSON(in, lib))
	}
	for _, p := range problems {
		doc.Warnings = append(doc.Warnings, Warning{Node: p[0], Message: p[1]})
	}
	return doc
}

func nodeJSON(g mix.Graph, n *mix.Node, lib presets.Presets) *NodeJSON {
	j := &NodeJSON{Name: n.Name, Kind: n.Kind, Bus: n.Bus, Pan: n.Pan, Preset: n.Preset, Plugin: n.Plugin, Insert: n.Insert, Sends: n.Sends}
	for _, k := range g.Children(n.Name) {
		j.Children = append(j.Children, nodeJSON(g, k, lib))
	}
	for _, t := range n.Tracks {
		j.Tracks = append(j.Tracks, trackJSON(t, lib))
	}
	return j
}

func trackJSON(in sheet.Input, lib presets.Presets) TrackJSON {
	rel, _ := lib.Find("Track", in.Label)
	t := TrackJSON{Label: in.Label, Musician: in.Musician, Source: in.SoundSource, Mic: in.Mic, Preamp: in.Preamp,
		HostIn: in.HostIn, Stem: in.Stem, ChannelPreset: rel}
	if !in.Stem {
		t.Interface = in.Source
	}
	return t
}

// InputsDoc is the flat list of sheet inputs (one row per known interface input), for tables.
type InputsDoc struct {
	Version   int         `json:"version"`
	Generated time.Time   `json:"generated"`
	Rig       string      `json:"rig,omitempty"`
	Inputs    []InputJSON `json:"inputs"`
}

type InputJSON struct {
	TrackJSON
	Row     int    `json:"row"`
	Checked bool   `json:"checked"`
	Stack   string `json:"stack,omitempty"`
}

// Inputs renders the flat input list.
func Inputs(rig string, inputs []sheet.Input, lib presets.Presets) InputsDoc {
	doc := InputsDoc{Version: Version, Generated: time.Now(), Rig: rig, Inputs: []InputJSON{}}
	for _, in := range inputs {
		doc.Inputs = append(doc.Inputs, InputJSON{TrackJSON: trackJSON(in, lib), Row: in.Row, Checked: in.Active, Stack: in.Stack})
	}
	return doc
}
