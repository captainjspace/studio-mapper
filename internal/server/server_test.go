package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/engine/internal/config"
	"studio/engine/internal/export"
	"studio/engine/internal/mix"
)

// testRepo writes a minimal config + sheet + preset index, with no preset library on disk (like the container).
func testRepo(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		config.Name: `{"preset_index": "data/presets.txt", "preset_dir": "/nonexistent", "default_rig": "oakland",
			"rigs": {
				"oakland": {"sheet": "data/inputs.csv", "interfaces": {"Motu 16A": "16a"},
					"host_inputs": [{"device": "16a", "bank": "Analog", "count": 16, "host_start": 1}]},
				"home": {"sheet": "data/home.csv", "interfaces": {"Motu UltraLite AVB": "ultralite"},
					"host_inputs": [{"device": "ultralite", "bank": "Analog", "count": 2, "host_start": 5}]}},
			"mix": {"Mix_Bus": {"kind": "aux"}, "Stereo_Out": {"kind": "output"}}}`,
		"data/home.csv":    "Checked?,Musician,Sound Source,Interface,Interface Input,Label,Stack\nTRUE,Josh,Vocals,Motu UltraLite AVB,Analog 1,UA_Ch1,Screaming_Demons\n",
		"data/inputs.csv":  "Checked?,Musician,Sound Source,Interface,Interface Input,Label,Stack\nTRUE,Erick,Kick In,Motu 16A,A3,Kick_In,drums/kick\n",
		"data/presets.txt": "Channel Strip Settings/Bus/drums.cst\nChannel Strip Settings/Track/Kick In.cst\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, _, err := config.Resolve(filepath.Join(dir, config.Name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestServer(t *testing.T) {
	srv := httptest.NewServer(New(testRepo(t)))
	defer srv.Close()

	if code, body := get(t, srv, "/healthz"); code != 200 || body != "ok\n" {
		t.Errorf("/healthz = %d %q", code, body)
	}
	if code, body := get(t, srv, "/"); code != 200 || !strings.Contains(body, "<title>Studio Map</title>") {
		t.Errorf("/ should serve the docs page, got %d", code)
	}

	code, body := get(t, srv, "/api/routing")
	var doc export.RoutingDoc
	if err := json.Unmarshal([]byte(body), &doc); code != 200 || err != nil {
		t.Fatalf("/api/routing = %d %v: %s", code, err, body)
	}
	kick := findNode(doc.Root, "kick")
	if doc.Version != 1 || doc.Root.Name != mix.StereoOut || kick == nil || kick.Tracks[0].ChannelPreset != "Track/Kick In.cst" {
		t.Errorf("routing doc should come from the sheet with presets matched via the index, got %+v", doc.Root)
	}
	if drums := findNode(doc.Root, "drums"); drums == nil || drums.Preset != "Bus/drums.cst" {
		t.Errorf("bus preset should match from the index, got %+v", drums)
	}

	if code, body := get(t, srv, "/studio-data.js"); code != 200 || !strings.HasPrefix(body, "window.STUDIO = {") {
		t.Errorf("/studio-data.js = %d %.40q", code, body)
	}
	code, body = get(t, srv, "/api/inputs")
	var inputs export.InputsDoc
	if err := json.Unmarshal([]byte(body), &inputs); code != 200 || err != nil || len(inputs.Inputs) != 1 {
		t.Fatalf("/api/inputs = %d %v: %s", code, err, body)
	}
	if in := inputs.Inputs[0]; !in.Checked || in.HostIn != 3 || in.Stack != "drums/kick" || in.Musician != "Erick" || in.ChannelPreset != "Track/Kick In.cst" {
		t.Errorf("input row = %+v", in)
	}
	code, body = get(t, srv, "/api/inputs?rig=home")
	var home export.InputsDoc
	if err := json.Unmarshal([]byte(body), &home); code != 200 || err != nil || home.Rig != "home" || len(home.Inputs) != 1 || home.Inputs[0].HostIn != 5 {
		t.Errorf("/api/inputs?rig=home = %d %v: %s", code, err, body)
	}
	if code, body := get(t, srv, "/api/rigs"); code != 200 || !strings.Contains(body, `"default": "oakland"`) || !strings.Contains(body, `"home"`) {
		t.Errorf("/api/rigs = %d %s", code, body)
	}
	if code, _ := get(t, srv, "/api/routing?rig=nowhere"); code != 500 {
		t.Errorf("unknown rig should fail, got %d", code)
	}
	if code, _ := get(t, srv, "/nope"); code != 404 {
		t.Errorf("unknown path should 404, got %d", code)
	}
}

func findNode(n *export.NodeJSON, name string) *export.NodeJSON {
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
