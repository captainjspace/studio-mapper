package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Input is one row of the studio sheet resolved against the host_inputs layout.
type Input struct {
	Row    int
	Device string
	Bank   string
	Ch     int // 1-based channel within the bank
	HostIn int // 0 when the bank/channel is not routed to the host
	Label  string
	Stack  string
	Active bool
	Source string
	Stem   bool // a Stem Splitter output, not a hardware input

	Musician    string
	SoundSource string
	Mic         string
	Preamp      string
}

// where names the track's source for build sheets: "In 5" or "Stem".
func (in Input) where() string {
	if in.Stem {
		return "Stem"
	}
	return fmt.Sprintf("In %d", in.HostIn)
}

// stemOrder is the order Logic's Stem Splitter outputs are listed in build sheets.
var stemOrder = []string{"Drums", "Bass", "Guitar", "Piano", "Vocals", "Other"}

// stemInputs turns cfg.StemSplit into tracks, one per Stem Splitter output, each on its stack.
func stemInputs(cfg Config) []Input {
	var inputs []Input
	for _, stem := range stemOrder {
		if stack, ok := cfg.StemSplit[stem]; ok {
			inputs = append(inputs, Input{Label: "Split_" + stem, Stack: stack, Active: true, Stem: true, Source: "Stem Splitter " + stem, SoundSource: stem})
		}
	}
	return inputs
}

// inputNotations decode the sheet's "Interface Input" column.
var inputNotations = []struct {
	re   *regexp.Regexp
	bank string
	ch   func(n []int) int
}{
	{regexp.MustCompile(`^A(\d+)$`), "Analog", func(n []int) int { return n[0] }},
	{regexp.MustCompile(`^AVB(\d+)-(\d+)$`), "Analog", func(n []int) int { return (n[0]-1)*8 + n[1] }},
	{regexp.MustCompile(`^O(\d+)$`), "Optical", func(n []int) int { return n[0] }},
	{regexp.MustCompile(`^(?i:mic|in)\s*(\d+)$`), "Mic/Inst", func(n []int) int { return n[0] }},
}

func decodeInterfaceInput(s string) (bank string, ch int, ok bool) {
	s = strings.TrimSpace(s)
	for _, n := range inputNotations {
		m := n.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		nums := make([]int, len(m)-1)
		for i, g := range m[1:] {
			nums[i], _ = strconv.Atoi(g)
		}
		return n.bank, n.ch(nums), true
	}
	return "", 0, false
}

func hostInFor(cfg Config, device, bank string, ch int) int {
	for _, h := range cfg.HostInputs {
		if h.Device == device && h.Bank == bank && ch >= 1 && ch <= h.Count {
			return h.HostStart + ch - 1
		}
	}
	return 0
}

// LoadSheet reads the CSV export of the studio Google Sheet.
// Rows whose Interface is not listed in cfg.Interfaces are ignored.
func LoadSheet(path string, cfg Config) ([]Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sheet %s: %w (set \"sheet\" in %s or pass --sheet)", path, err, configName)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}

	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.TrimSpace(h)] = i
	}
	for _, required := range []string{"Checked?", "Interface", "Interface Input"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("%s: missing column %q", path, required)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var inputs []Input
	for i, rec := range records[1:] {
		iface, raw := get(rec, "Interface"), get(rec, "Interface Input")
		device, known := cfg.Interfaces[iface]
		if !known {
			continue
		}
		bank, ch, ok := decodeInterfaceInput(raw)
		if !ok {
			continue
		}
		label := get(rec, "Label")
		if label == "" {
			label = strings.Trim(get(rec, "Musician")+" "+get(rec, "Sound Source"), " ")
		}
		inputs = append(inputs, Input{
			Row:    i + 2,
			Device: device,
			Bank:   bank,
			Ch:     ch,
			HostIn: hostInFor(cfg, device, bank, ch),
			Label:  SanitizeHardwareName(label),
			Stack:  get(rec, "Stack"),
			Active: strings.EqualFold(get(rec, "Checked?"), "TRUE"),
			Source: iface + " " + raw,

			Musician:    get(rec, "Musician"),
			SoundSource: get(rec, "Sound Source"),
			Mic:         get(rec, "Device Source"),
			Preamp:      get(rec, "Preamp"),
		})
	}
	return inputs, nil
}
