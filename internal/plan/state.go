package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"studio/engine/internal/motu"
)

// Snapshot records a device's names and routes before apply, so they can be restored.
type Snapshot struct {
	Device string            `json:"device"`
	URL    string            `json:"url"`
	Taken  time.Time         `json:"taken"`
	Names  map[string]string `json:"names"` // channel names and router sources, by datastore key
}

// SaveSnapshot writes <dir>/<device>-<time>.json.
func SaveSnapshot(dir, device, url string, ds motu.Datastore) (string, error) {
	snap := Snapshot{Device: device, URL: url, Taken: time.Now(), Names: ds.Settings()}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.json", device, snap.Taken.Format("20060102-150405")))
	data, _ := json.MarshalIndent(snap, "", "  ")
	return path, os.WriteFile(path, data, 0o644)
}

func LoadSnapshot(path string) (Snapshot, error) {
	var snap Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, fmt.Errorf("parse %s: %w", path, err)
	}
	return snap, nil
}
