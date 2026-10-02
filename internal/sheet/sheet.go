// Package sheet reads the studio Google Sheet (CSV export) into inputs resolved against a rig's Host In layout.
package sheet

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"studio/engine/internal/config"
	"studio/engine/internal/motu"
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

// Where names the track's source for build sheets: "In 5" or "Stem".
func (in Input) Where() string {
	if in.Stem {
		return "Stem"
	}
	return fmt.Sprintf("In %d", in.HostIn)
}

// stemOrder is the order Logic's Stem Splitter outputs are listed in build sheets.
var stemOrder = []string{"Drums", "Bass", "Guitar", "Piano", "Vocals", "Other"}

// StemInputs turns cfg.StemSplit into tracks, one per Stem Splitter output, each on its stack.
func StemInputs(cfg config.Config) []Input {
	var inputs []Input
	for _, stem := range stemOrder {
		if stack, ok := cfg.StemSplit[stem]; ok {
			inputs = append(inputs, Input{Label: "Split_" + stem, Stack: stack, Active: true, Stem: true, Source: "Stem Splitter " + stem, SoundSource: stem})
		}
	}
	return inputs
}

// ActiveInputs returns the checked rows in Host In order.
func ActiveInputs(inputs []Input) []Input {
	var out []Input
	for _, in := range inputs {
		if in.Active {
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].HostIn < out[j].HostIn })
	return out
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
}

// DecodeInterfaceInput reads "A5", "AVB1-3", "O1" or the generic "<Bank> N" ("Analog 1", "Mic 3").
func DecodeInterfaceInput(s string) (bank string, ch int, ok bool) {
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
	return motu.ChannelRef(s)
}

func hostInFor(cfg config.Config, device, bank string, ch int) int {
	for _, h := range cfg.HostInputs {
		from := cmp.Or(h.From, 1)
		if h.Device == device && h.Bank == bank && ch >= from && ch < from+h.Count {
			return h.HostStart + ch - from
		}
	}
	return 0
}

// Load reads the CSV export of the studio Google Sheet.
// Rows whose Interface is not listed in cfg.Interfaces are ignored.
func Load(path string, cfg config.Config) ([]Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sheet %s: %w (set \"sheet\" in %s or pass --sheet)", path, err, config.Name)
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
		bank, ch, ok := DecodeInterfaceInput(raw)
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
			Label:  SanitizeName(label),
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

// SanitizeName cleans strings to prevent MOTU hardware internal errors: underscores for spaces,
// letters/digits/_/- only, at most 31 characters.
func SanitizeName(rawName string) string {
	spaced := strings.ReplaceAll(rawName, " ", "_")
	sanitized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			return r
		}
		return -1
	}, spaced)
	if len(sanitized) > 31 {
		return sanitized[:31]
	}
	return sanitized
}
