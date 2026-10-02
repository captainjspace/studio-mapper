package config

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"studio/engine/internal/coreaudio"
)

// ChooseRig picks the rig: --rig, then $STUDIO_MAP_RIG, then detection (when asked), then the default.
func ChooseRig(c Config, flag string, detect bool) (string, error) {
	if name := cmp.Or(flag, os.Getenv("STUDIO_MAP_RIG")); name != "" {
		return name, nil
	}
	if !detect {
		return c.DefaultRigName(), nil
	}
	return DetectRig(c, &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}})
}

// DetectRig returns the one rig whose devices answer: a datastore device on the network or its CoreAudio host device.
func DetectRig(c Config, client *http.Client) (string, error) {
	var found []string
	for _, name := range c.RigNames() {
		if rigPresent(c.Rigs[name], client) {
			found = append(found, name)
		}
	}
	if len(found) == 1 {
		fmt.Printf("📍 rig: %s (detected)\n", found[0])
		return found[0], nil
	}
	what := "none of the rigs' devices answered"
	if len(found) > 1 {
		what = "more than one rig answered: " + strings.Join(found, ", ")
	}
	return "", fmt.Errorf("can't tell which rig this is (%s); pass --rig %s", what, strings.Join(c.RigNames(), "|"))
}

func rigPresent(r Rig, client *http.Client) bool {
	for _, url := range r.Devices {
		if url == "" {
			continue
		}
		if resp, err := client.Get(url + "/datastore/uid"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
	}
	if r.HostDevice != "" {
		if names, err := coreaudio.HostInNames(r.HostDevice); err == nil && len(names) > 0 {
			return true
		}
	}
	return false
}
