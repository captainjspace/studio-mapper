package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Snapshot records a device's input names before apply, so they can be restored.
type Snapshot struct {
	Device string            `json:"device"`
	URL    string            `json:"url"`
	Taken  time.Time         `json:"taken"`
	Names  map[string]string `json:"names"`
}

func saveSnapshot(dir, device, url string, ds Datastore) (string, error) {
	snap := Snapshot{Device: device, URL: url, Taken: time.Now(), Names: ds.inputNames()}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.json", device, snap.Taken.Format("20060102-150405")))
	data, _ := json.MarshalIndent(snap, "", "  ")
	return path, os.WriteFile(path, data, 0o644)
}

func loadSnapshot(path string) (Snapshot, error) {
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

// changedNames returns the entries of want that differ from the live datastore.
func changedNames(ds Datastore, want map[string]string) map[string]string {
	diff := map[string]string{}
	for k, v := range want {
		if ds.str(k) != v {
			diff[k] = v
		}
	}
	return diff
}
