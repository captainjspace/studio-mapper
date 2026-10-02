package plan

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studio/engine/internal/config"
	"studio/engine/internal/motu"
	"studio/engine/internal/motu/motutest"
)

// newFakeUltraLite serves a small UltraLite-like datastore: inputs, an aux bus, computer loopbacks.
func newFakeUltraLite() *motutest.Fake {
	ds := motu.Datastore{
		"uid":              "0001f2fffe004524",
		"ext/ibank/0/name": "Analog", "ext/ibank/1/name": "Mix Aux", "ext/ibank/2/name": "Computer",
		"ext/obank/0/name": "Analog", "ext/obank/1/name": "Computer",
		"ext/obank/0/ch/0/name": "", "ext/obank/0/ch/0/src": "",
		"ext/obank/1/ch/0/src": "0:0", // To Computer 1 <- Analog 1
		"ext/obank/1/ch/1/src": "2:1", // To Computer 2 <- From Computer 2 (undeclared loopback)
		"ext/obank/1/ch/2/src": "2:2", // To Computer 3 <- From Computer 3 (declared print path)
	}
	return &motutest.Fake{DS: ds}
}

func TestPlanOutputsRoutesApplyRestore(t *testing.T) {
	fake := newFakeUltraLite()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client := &http.Client{Timeout: time.Second}
	cfg := config.Config{
		Devices: map[string]string{"ultralite": srv.URL},
		Outputs: map[string]map[string]string{"ultralite": {"Analog 1": "1176 Send L"}},
		Routes: map[string]map[string]string{"ultralite": {
			"Analog 1": "Mix Aux 5", "Computer 3": "Computer 3", "Computer 9": "Nowhere 1"}},
	}
	read := func() Live {
		ds, err := motu.Fetch(client, srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		return Live{Datastores: map[string]motu.Datastore{"ultralite": ds}}
	}

	before := read()
	p := Build(cfg, nil, before)
	steps := map[string]Step{}
	for _, s := range p.Steps {
		steps[s.Scope+"|"+s.Target] = s
	}
	if s := steps["ultralite outputs|Analog 1"]; s.Status != StatusChange || s.Desired != "1176_Send_L" {
		t.Errorf("output name step = %+v", s)
	}
	if s := steps["ultralite routes|Analog 1 ←"]; s.Status != StatusChange || s.Current != "(none)" || s.Desired != "Mix Aux 5" {
		t.Errorf("route step = %+v", s)
	}
	if s := steps["ultralite routes|Computer 3 ←"]; s.Status != StatusOK {
		t.Errorf("declared loopback should already match, got %+v", s)
	}
	if s := steps["ultralite routes|Computer 9 ← Nowhere 1"]; s.Status != StatusWarn {
		t.Errorf("unknown source bank should warn, got %+v", s)
	}
	if s := steps["ultralite routes|To Computer 2"]; s.Status != StatusWarn || !strings.Contains(s.Desired, "feedback") {
		t.Errorf("undeclared loopback should warn, got %+v", steps)
	}
	if got := p.Writes["ultralite"]; got["ext/obank/0/ch/0/src"] != "1:4" || got["ext/obank/0/ch/0/name"] != "1176_Send_L" {
		t.Errorf("writes = %v", got)
	}

	snapPath, err := SaveSnapshot(t.TempDir(), "ultralite", srv.URL, before.Datastores["ultralite"])
	if err != nil {
		t.Fatal(err)
	}
	if err := motu.WriteKeys(client, srv.URL, p.Writes["ultralite"]); err != nil {
		t.Fatal(err)
	}
	if again := Build(cfg, nil, read()); again.Count(StatusChange) != 0 {
		t.Errorf("plan after apply should be clean, got %v", again.Writes)
	}

	snap, err := LoadSnapshot(snapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := motu.WriteKeys(client, srv.URL, read().Datastores["ultralite"].Changed(snap.Names)); err != nil {
		t.Fatal(err)
	}
	if fake.Get("ext/obank/0/ch/0/src") != "" || fake.Get("ext/obank/0/ch/0/name") != "" {
		t.Errorf("restore should put the route and name back, got src=%v name=%v", fake.Get("ext/obank/0/ch/0/src"), fake.Get("ext/obank/0/ch/0/name"))
	}
}

func TestCompactRanges(t *testing.T) {
	if got := compactRanges([]int{11, 12, 17, 18, 19, 33}); got != "11–12, 17–19, 33" {
		t.Errorf("got %q", got)
	}
}
