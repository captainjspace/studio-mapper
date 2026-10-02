package main

import "time"

// routingDocVersion is bumped on breaking changes to the JSON the band app reads.
const routingDocVersion = 1

type RoutingDoc struct {
	Version     int         `json:"version"`
	Generated   time.Time   `json:"generated"`
	Mode        string      `json:"mode"` // "session" or "stems"
	Root        *NodeJSON   `json:"root,omitempty"`
	NotRecorded []TrackJSON `json:"notRecorded,omitempty"`
	Warnings    []Warning   `json:"warnings"`
}

type NodeJSON struct {
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Bus      int         `json:"bus,omitempty"`
	Pan      *int        `json:"pan,omitempty"`
	Preset   string      `json:"preset,omitempty"`
	Plugin   string      `json:"plugin,omitempty"`
	Insert   *Insert     `json:"insert,omitempty"`
	Sends    []Send      `json:"sends,omitempty"`
	Children []*NodeJSON `json:"children,omitempty"`
	Tracks   []TrackJSON `json:"tracks,omitempty"`
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

// routingDoc renders the mix graph as the nested tree the band app displays.
func routingDoc(g MixGraph, presets Presets, mode string, problems [][2]string) RoutingDoc {
	doc := RoutingDoc{Version: routingDocVersion, Generated: time.Now(), Mode: mode, Warnings: []Warning{}}
	if out, ok := g.Nodes[stereoOut]; ok {
		doc.Root = nodeJSON(g, out, presets)
	}
	for _, in := range g.Utility {
		doc.NotRecorded = append(doc.NotRecorded, trackJSON(in, presets))
	}
	for _, p := range problems {
		doc.Warnings = append(doc.Warnings, Warning{Node: p[0], Message: p[1]})
	}
	return doc
}

func nodeJSON(g MixGraph, n *Node, presets Presets) *NodeJSON {
	j := &NodeJSON{Name: n.Name, Kind: n.Kind, Bus: n.Bus, Pan: n.Pan, Preset: n.Preset, Plugin: n.Plugin, Insert: n.Insert, Sends: n.Sends}
	for _, k := range g.children(n.Name) {
		j.Children = append(j.Children, nodeJSON(g, k, presets))
	}
	for _, t := range n.Tracks {
		j.Tracks = append(j.Tracks, trackJSON(t, presets))
	}
	return j
}

func trackJSON(in Input, presets Presets) TrackJSON {
	rel, _ := presets.find("Track", in.Label)
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
	Inputs    []InputJSON `json:"inputs"`
}

type InputJSON struct {
	TrackJSON
	Row     int    `json:"row"`
	Checked bool   `json:"checked"`
	Stack   string `json:"stack,omitempty"`
}

func inputsDoc(inputs []Input, presets Presets) InputsDoc {
	doc := InputsDoc{Version: routingDocVersion, Generated: time.Now(), Inputs: []InputJSON{}}
	for _, in := range inputs {
		doc.Inputs = append(doc.Inputs, InputJSON{TrackJSON: trackJSON(in, presets), Row: in.Row, Checked: in.Active, Stack: in.Stack})
	}
	return doc
}
