package plan

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"studio/engine/internal/config"
	"studio/engine/internal/coreaudio"
	"studio/engine/internal/mix"
	"studio/engine/internal/motu"
	"studio/engine/internal/presets"
)

// ReadLive queries every device in the rig's device map, plus the host device's CoreAudio names.
func ReadLive(client *http.Client, cfg config.Config, p config.Paths) Live {
	live := Live{Datastores: map[string]motu.Datastore{}, Presets: presets.For(p, cfg), Known: mix.KnownStacks(cfg, p)}
	for _, dev := range slices.Sorted(maps.Keys(cfg.Devices)) {
		url := cfg.Devices[dev]
		if url == "" {
			continue
		}
		ds, err := motu.Fetch(client, url)
		if err != nil {
			fmt.Printf("⚠️  %s unreachable (%v)\n", dev, err)
			continue
		}
		fmt.Printf("📡 %s: read %d datastore keys\n", dev, len(ds))
		live.Datastores[dev] = ds
	}
	if names, err := coreaudio.HostInNames(cfg.HostDevice); err == nil {
		fmt.Printf("🎧 %s: read %d CoreAudio input names\n", cfg.HostDevice, len(names))
		live.HostNames = names
	}
	return live
}

// Apply snapshots each device it changes, writes the plan's differences, and verifies them.
func Apply(client *http.Client, cfg config.Config, p config.Paths, pl Plan, live Live) error {
	for _, dev := range slices.Sorted(maps.Keys(pl.Writes)) {
		writes := pl.Writes[dev]
		url := cfg.Devices[dev]
		snap, err := SaveSnapshot(p.State, dev, url, live.Datastores[dev])
		if err != nil {
			return fmt.Errorf("snapshot %s: %w", dev, err)
		}
		fmt.Printf("\n💾 %s snapshot: %s\n", dev, snap)
		if err := motu.WriteKeys(client, url, writes); err != nil {
			return fmt.Errorf("write %s: %w", dev, err)
		}
		if err := motu.VerifyWrites(client, url, writes); err != nil {
			return fmt.Errorf("verify %s: %w", dev, err)
		}
		fmt.Printf("✅ %s: %d settings written and verified\n", dev, len(writes))
	}
	if n := pl.Count(StatusManual); n > 0 {
		fmt.Printf("\n✋ %d items to do by hand in CueMix/Logic (see ✋ rows above).\n", n)
	}
	return nil
}

// Restore writes a snapshot back to its device (found in any rig, or the URL the snapshot recorded).
func Restore(client *http.Client, cfg config.Config, path string) error {
	snap, err := LoadSnapshot(path)
	if err != nil {
		return err
	}
	url := cmp.Or(cfg.DeviceURL(snap.Device), snap.URL)
	ds, err := motu.Fetch(client, url)
	if err != nil {
		return fmt.Errorf("%s unreachable: %w", snap.Device, err)
	}
	diff := ds.Changed(snap.Names)
	if err := motu.WriteKeys(client, url, diff); err != nil {
		return err
	}
	if err := motu.VerifyWrites(client, url, diff); err != nil {
		return err
	}
	fmt.Printf("⏪ %s: restored %d settings from %s\n", snap.Device, len(diff), snap.Taken.Format(time.RFC3339))
	return nil
}
