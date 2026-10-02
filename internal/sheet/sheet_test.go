package sheet

import (
	"os"
	"path/filepath"
	"testing"

	"studio/engine/internal/config"
)

func TestDecodeInterfaceInput(t *testing.T) {
	cases := []struct {
		in   string
		bank string
		ch   int
		ok   bool
	}{
		{"A5", "Analog", 5, true},
		{"AVB1-1", "Analog", 1, true},
		{"AVB3-8", "Analog", 24, true},
		{"O3", "Optical", 3, true},
		{"Mic 2", "Mic", 2, true},
		{"Analog 1", "Analog", 1, true},
		{"Mix Aux 5", "Mix Aux", 5, true},
		{"Input-Left", "", 0, false},
	}
	for _, c := range cases {
		bank, ch, ok := DecodeInterfaceInput(c.in)
		if bank != c.bank || ch != c.ch || ok != c.ok {
			t.Errorf("%q → (%q, %d, %t), want (%q, %d, %t)", c.in, bank, ch, ok, c.bank, c.ch, c.ok)
		}
	}
}

const fixtureCSV = `Checked?,Musician,Sound Source,Interface,Interface Input,Label,Stack
TRUE,Erick,Kick In,Motu 16A,A3,Kick_In,drums/kick
TRUE,Josh,Guitar Direct Left,Motu 24Ai,AVB3-5,Fractal_L,Smashed_Guitars/joshgtr
TRUE,Erick,Tom 1,Motu 24Ai,AVB1-1,Tom1,drums/toms
FALSE,Josh,Vocals,Motu 16A,A1,Josh_Vox,Screaming_Demons
TRUE,Bill ,Bass Direct,Motu 16A,O1,,Punchy_Bass
TRUE,MAINS,Headphone L,Behringer Monitor 1,Input Left,,
`

func TestLoadSheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inputs.csv")
	if err := os.WriteFile(path, []byte(fixtureCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Interfaces: map[string]string{"Motu 16A": "16a", "Motu 24Ai": "24ai"},
		HostInputs: []config.HostInput{
			{Device: "16a", Bank: "Analog", Count: 16, HostStart: 1},
			{Device: "24ai", Bank: "Analog", Count: 24, HostStart: 17},
		},
	}
	inputs, err := Load(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 5 {
		t.Fatalf("want 5 rows (unknown interface skipped), got %d: %+v", len(inputs), inputs)
	}
	want := []struct {
		label  string
		hostIn int
		active bool
	}{
		{"Kick_In", 3, true},
		{"Fractal_L", 37, true},
		{"Tom1", 17, true},
		{"Josh_Vox", 1, false},
		{"Bill_Bass_Direct", 0, true}, // blank Label falls back to Musician + Sound Source; optical is unrouted
	}
	for i, w := range want {
		in := inputs[i]
		if in.Label != w.label || in.HostIn != w.hostIn || in.Active != w.active {
			t.Errorf("row %d = %s/Host In %d/active %t, want %s/%d/%t", in.Row, in.Label, in.HostIn, in.Active, w.label, w.hostIn, w.active)
		}
	}
}

// TestStudioSheetLoads guards the real sheet export: it must parse against the real config.
func TestStudioSheetLoads(t *testing.T) {
	paths, cfg, err := config.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	for _, rig := range cfg.RigNames() {
		rc, err := cfg.WithRig(rig)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Load(paths.Abs(rc.Sheet), rc); err != nil {
			t.Errorf("rig %s: %v", rig, err)
		}
	}
}
