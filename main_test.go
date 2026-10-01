package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMOTU serves a MOTU AVB datastore: GET /datastore returns every key, POST json={...} sets keys.
type fakeMOTU struct {
	mu sync.Mutex
	ds Datastore
}

func (f *fakeMOTU) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/datastore" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		var set map[string]string
		if err := json.Unmarshal([]byte(r.FormValue("json")), &set); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for k, v := range set {
			f.ds[k] = v
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(w).Encode(f.ds)
}

const (
	uid16a   = "0001f2fffe0011e5"
	uid10pre = "0001f2fffefe96ae"
)

func newFake16A(streamTalkers ...string) *fakeMOTU {
	ds := Datastore{
		"uid":                              uid16a,
		"ext/ibank/0/name":                 "Analog",
		"ext/ibank/1/name":                 "ADAT A",
		"avb/" + uid10pre + "/entity_name": "10pre",
	}
	for ch := 0; ch < 16; ch++ {
		ds[inputNamePath(0, ch+1)] = ""
	}
	for i, t := range streamTalkers {
		ds[fmtTalkerKey(i)] = t
	}
	return &fakeMOTU{ds: ds}
}

func fmtTalkerKey(i int) string {
	return "avb/" + uid10pre + "/cfg/0/input_streams/" + string(rune('0'+i)) + "/talker"
}

func testConfig(url string) Config {
	return Config{
		Devices:    map[string]string{"16a": url, "10pre": ""},
		Interfaces: map[string]string{"Motu 16A": "16a"},
		HostDevice: "10pre",
		HostInputs: []HostInput{{Device: "16a", Bank: "Analog", Count: 16, HostStart: 1, AVBStream: 1}},
	}
}

var testInputs = []Input{
	{Row: 2, Device: "16a", Bank: "Analog", Ch: 5, HostIn: 5, Label: "Kick_In", Stack: "drums/kick", Active: true},
	{Row: 3, Device: "16a", Bank: "Analog", Ch: 6, HostIn: 6, Label: "Snare_Top", Stack: "drums/snare", Active: true},
	{Row: 4, Device: "16a", Bank: "Analog", Ch: 16, HostIn: 16, Label: "Spare", Active: false},
}

func TestPlanApplyRestore(t *testing.T) {
	fake := newFake16A(uid16a+":0", uid16a+":1")
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client := &http.Client{Timeout: time.Second}
	cfg := testConfig(srv.URL)

	read := func() Live {
		ds, err := fetchDatastore(client, srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		return Live{Datastores: map[string]Datastore{"16a": ds}, HostNames: []string{"Host In 1", "Host In 2", "Host In 3", "Host In 4", "Kick_In", "Host In 6"}}
	}

	before := read()
	p := buildPlan(cfg, testInputs, before)
	if got := p.count(StatusChange); got != 2 {
		t.Fatalf("want 2 changes (inactive row skipped), got %d: %+v", got, p.Steps)
	}
	if got := p.count(StatusManual); got != 1 {
		t.Errorf("want 1 manual Host In rename (Host In 6), got %d", got)
	}
	if got := p.count(StatusOK); got != 3 {
		t.Errorf("want 2 streams + Host In 5 ok, got %d ok", got)
	}

	snapPath, err := saveSnapshot(t.TempDir(), "16a", srv.URL, before.Datastores["16a"])
	if err != nil {
		t.Fatal(err)
	}
	if err := writeKeys(client, srv.URL, p.Writes["16a"]); err != nil {
		t.Fatal(err)
	}
	if err := verifyWrites(client, srv.URL, p.Writes["16a"]); err != nil {
		t.Fatal(err)
	}
	if got := fake.ds[inputNamePath(0, 5)]; got != "Kick_In" {
		t.Errorf("Analog 5 = %q, want Kick_In", got)
	}

	if again := buildPlan(cfg, testInputs, read()); again.count(StatusChange) != 0 {
		t.Errorf("plan after apply should be clean, got %+v", again.Writes)
	}

	snap, err := loadSnapshot(snapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeKeys(client, srv.URL, changedNames(read().Datastores["16a"], snap.Names)); err != nil {
		t.Fatal(err)
	}
	if got := fake.ds[inputNamePath(0, 5)]; got != "" {
		t.Errorf("restore should blank Analog 5, got %q", got)
	}
}

func TestPlanFlagsDisconnectedStream(t *testing.T) {
	fake := newFake16A(uid16a+":0", "0000000000000000:0")
	srv := httptest.NewServer(fake)
	defer srv.Close()
	ds, _ := fetchDatastore(&http.Client{Timeout: time.Second}, srv.URL)

	p := buildPlan(testConfig(srv.URL), nil, Live{Datastores: map[string]Datastore{"16a": ds}})
	var manual []Step
	for _, s := range p.Steps {
		if s.Status == StatusManual {
			manual = append(manual, s)
		}
	}
	if len(manual) != 1 || manual[0].Target != "Input Stream 2" || manual[0].Current != "(not connected)" {
		t.Errorf("want Input Stream 2 flagged as not connected, got %+v", manual)
	}
}

func TestPlanSurvivesOfflineDevice(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1")
	live := readLive(&http.Client{Timeout: 200 * time.Millisecond}, cfg)
	p := buildPlan(cfg, testInputs, live)
	if p.count(StatusChange) != 0 || len(p.Writes) != 0 {
		t.Errorf("offline device must produce no writes, got %+v", p.Writes)
	}
	found := false
	for _, s := range p.Steps {
		found = found || (s.Status == StatusWarn && strings.Contains(s.Desired, "offline"))
	}
	if !found {
		t.Error("expected an offline warning")
	}
}

func TestPlanValidation(t *testing.T) {
	cfg := testConfig("")
	inputs := []Input{
		{Row: 2, HostIn: 5, Label: "A", Stack: "drums", Active: true},
		{Row: 3, HostIn: 5, Label: "B", Stack: "drums", Active: true},
		{Row: 4, HostIn: 0, Label: "Bass_Direct", Source: "Motu 16A O1", Active: true},
	}
	p := buildPlan(cfg, inputs, Live{})
	var warns []string
	for _, s := range p.Steps {
		if s.Status == StatusWarn && s.Scope == "sheet" {
			warns = append(warns, s.Desired)
		}
	}
	want := []string{"Bass_Direct: not routed to the host", "B: duplicate Host In"}
	if strings.Join(warns, "|") != strings.Join(want, "|") {
		t.Errorf("warnings = %q, want %q", warns, want)
	}
}
