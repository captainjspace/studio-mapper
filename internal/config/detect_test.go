package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studio/engine/internal/motu"
	"studio/engine/internal/motu/motutest"
)

func TestDetectRig(t *testing.T) {
	home := httptest.NewServer(&motutest.Fake{DS: motu.Datastore{"uid": "0001f2fffe004524"}})
	defer home.Close()
	client := &http.Client{Timeout: time.Second}
	cfg := Config{Rigs: map[string]Rig{
		"oakland": {Devices: map[string]string{"16a": "http://127.0.0.1:1"}},
		"home":    {Devices: map[string]string{"ultralite": home.URL}},
	}}

	if got, err := DetectRig(cfg, client); err != nil || got != "home" {
		t.Errorf("detect = %q %v, want home", got, err)
	}
	cfg.Rigs["oakland"] = Rig{Devices: map[string]string{"16a": home.URL}}
	if _, err := DetectRig(cfg, client); err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Errorf("two answering rigs should ask for --rig, got %v", err)
	}
	cfg.Rigs = map[string]Rig{"oakland": {Devices: map[string]string{"16a": "http://127.0.0.1:1"}}}
	if _, err := DetectRig(cfg, client); err == nil || !strings.Contains(err.Error(), "--rig") {
		t.Errorf("nothing answering should ask for --rig, got %v", err)
	}
	t.Setenv("STUDIO_MAP_RIG", "home")
	if got, _ := ChooseRig(cfg, "", true); got != "home" {
		t.Errorf("STUDIO_MAP_RIG should win over detection, got %q", got)
	}
}
