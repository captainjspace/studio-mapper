package main

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Rig is one location's device map: what to query, how its inputs reach the host, and what to correct.
type Rig struct {
	Sheet      string                       `json:"sheet"`       // relative to the config file
	HostDevice string                       `json:"host_device"` // CoreAudio device Logic uses here
	Devices    map[string]string            `json:"devices"`     // name -> datastore URL ("" = no datastore, e.g. 10pre)
	Interfaces map[string]string            `json:"interfaces"`  // sheet "Interface" value -> device name
	HostInputs []HostInput                  `json:"host_inputs"`
	Outputs    map[string]map[string]string `json:"outputs,omitempty"` // device -> "Analog 1" -> name
	Routes     map[string]map[string]string `json:"routes,omitempty"`  // device -> destination "Analog 1" -> source "Mix Aux 5"
}

// withRig returns the effective config for one rig: the shared parts plus that rig's device map.
func (c Config) withRig(name string) (Config, error) {
	r, ok := c.Rigs[name]
	if !ok {
		return c, fmt.Errorf("no rig %q in config (have %s)", name, strings.Join(sortedKeys(c.Rigs), ", "))
	}
	c.RigName = name
	c.Devices, c.Interfaces, c.HostDevice, c.HostInputs = r.Devices, r.Interfaces, r.HostDevice, r.HostInputs
	c.Outputs, c.Routes = r.Outputs, r.Routes
	c.Sheet = cmp.Or(r.Sheet, "data/studio-inputs.csv")
	return c, nil
}

func (c Config) defaultRig() string {
	if c.DefaultRig != "" {
		return c.DefaultRig
	}
	if names := sortedKeys(c.Rigs); len(names) > 0 {
		return names[0]
	}
	return ""
}

// chooseRig picks the rig: --rig, then $STUDIO_MAP_RIG, then detection (when asked), then the default.
func chooseRig(c Config, flag string, detect bool) (string, error) {
	if name := cmp.Or(flag, os.Getenv("STUDIO_MAP_RIG")); name != "" {
		return name, nil
	}
	if !detect {
		return c.defaultRig(), nil
	}
	return detectRig(c, &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}})
}

// detectRig returns the one rig whose devices answer: a datastore device on the network or its CoreAudio host device.
func detectRig(c Config, client *http.Client) (string, error) {
	var found []string
	for _, name := range sortedKeys(c.Rigs) {
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
	return "", fmt.Errorf("can't tell which rig this is (%s); pass --rig %s", what, strings.Join(sortedKeys(c.Rigs), "|"))
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
		if names, err := hostInNames(r.HostDevice); err == nil && len(names) > 0 {
			return true
		}
	}
	return false
}

// deviceURL finds a device's URL in any rig (restore works wherever the snapshot came from).
func (c Config) deviceURL(device string) string {
	for _, r := range c.Rigs {
		if url := r.Devices[device]; url != "" {
			return url
		}
	}
	return ""
}
