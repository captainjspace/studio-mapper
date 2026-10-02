package mix

import (
	"fmt"
	"strings"

	"studio/engine/internal/presets"
)

func (n *Node) describe() string {
	var parts []string
	if n.Bus > 0 {
		parts = append(parts, fmt.Sprintf("[Bus %d]", n.Bus))
	}
	if n.Pan != nil {
		parts = append(parts, fmt.Sprintf("pan %+d", *n.Pan))
	}
	if p := cmpOrDash(n.Preset, n.Plugin); p != "—" {
		parts = append(parts, "⟵ "+p)
	}
	if n.Insert != nil {
		parts = append(parts, "⏱ ("+n.Insert.String()+")")
	}
	for _, s := range n.Sends {
		parts = append(parts, fmt.Sprintf("⤳ %s %.1fdB", s.To, s.Level))
	}
	return strings.Join(parts, "  ")
}

// PrintBlueprint prints the signal flow from Stereo_Out back to every track.
func PrintBlueprint(g Graph) {
	fmt.Println("\n🎛️  LOGIC SIGNAL FLOW")
	out, ok := g.Nodes[StereoOut]
	if !ok {
		fmt.Printf(" (no %s in mix config)\n", StereoOut)
		return
	}
	fmt.Printf(" 🔊 %s  %s\n", out.Name, out.describe())
	printBranch(g, out, " ")
}

func printBranch(g Graph, n *Node, indent string) {
	kids := g.Children(n.Name)
	for i, k := range kids {
		last := i == len(kids)-1 && len(n.Tracks) == 0
		fmt.Printf("%s%s 📂 %s  %s\n", indent, branch(last), k.Name, k.describe())
		printBranch(g, k, indent+stem(last))
	}
	for i, t := range n.Tracks {
		fmt.Printf("%s%s %-6s %s\n", indent, branch(i == len(n.Tracks)-1), t.Where(), t.Label)
	}
}

func branch(last bool) string {
	if last {
		return "└──"
	}
	return "├──"
}

func stem(last bool) string {
	if last {
		return "    "
	}
	return "│   "
}

// PrintRouting prints the build sheet: what to set on each strip in the Logic template.
func PrintRouting(g Graph, lib presets.Presets) {
	fmt.Println("\n── TRACKS (Input → Output, channel preset)")
	fmt.Printf("  %-6s %-16s %-24s %s\n", "Input", "Track", "Output", "Channel preset")
	for _, name := range g.Order {
		n := g.Nodes[name]
		for _, t := range n.Tracks {
			rel, _ := lib.Find("Track", t.Label)
			fmt.Printf("  %-6s %-16s %-24s %s\n", t.Where(), t.Label, busLabel(g, name), cmpOrDash(rel, ""))
		}
	}

	fmt.Println("\n── STACKS / AUX / OUTPUT (input bus → output)")
	fmt.Printf("  %-8s %-18s %-7s %-24s %-5s %s\n", "Input", "Name", "Kind", "Output", "Pan", "Preset / plugin")
	for _, name := range g.Order {
		n := g.Nodes[name]
		in, outTo, pan := busNum(n), "—", "—"
		if n.Kind != "output" {
			outTo = busLabel(g, n.Output)
		}
		if n.Pan != nil {
			pan = fmt.Sprintf("%+d", *n.Pan)
		}
		fmt.Printf("  %-8s %-18s %-7s %-24s %-5s %s\n", in, name, n.Kind, outTo, pan, cmpOrDash(n.Preset, n.Plugin))
	}

	if len(g.Utility) > 0 {
		fmt.Println("\n── NOT RECORDED (hardware label only, no Logic track)")
		for _, t := range g.Utility {
			fmt.Printf("  In %-3d %s\n", t.HostIn, t.Label)
		}
	}

	fmt.Println("\n── OUTBOARD (hardware inserts and sends)")
	for _, name := range g.Order {
		if n := g.Nodes[name]; n.Insert != nil {
			fmt.Printf("  %-18s %s  (%s)\n", name, n.Plugin, n.Insert)
		}
	}

	fmt.Println("\n── SENDS")
	for _, name := range g.Order {
		for _, s := range g.Nodes[name].Sends {
			fmt.Printf("  %-18s ⤳ %-24s %.1f dB\n", name, busLabel(g, s.To), s.Level)
		}
	}
}

func busNum(n *Node) string {
	if n.Kind == "output" {
		return "St Out"
	}
	return fmt.Sprintf("Bus %d", n.Bus)
}

func busLabel(g Graph, name string) string {
	n, ok := g.Nodes[name]
	switch {
	case !ok:
		return name + " (missing)"
	case n.Kind == "output":
		return busNum(n)
	default:
		return busNum(n) + " (" + name + ")"
	}
}

func cmpOrDash(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "—"
}
