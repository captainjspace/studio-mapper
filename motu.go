package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Datastore is the flat key/value tree a MOTU AVB device serves at /datastore.
type Datastore map[string]any

// settingKey matches what apply can change and restore puts back: channel names and router sources.
var settingKey = regexp.MustCompile(`^ext/[io]bank/\d+/ch/\d+/(name|src)$`)

func fetchDatastore(client *http.Client, baseURL string) (Datastore, error) {
	resp, err := client.Get(baseURL + "/datastore")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s/datastore: %s", baseURL, resp.Status)
	}
	var ds Datastore
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return nil, fmt.Errorf("decode %s/datastore: %w", baseURL, err)
	}
	return ds, nil
}

// writeKeys sets several datastore keys in a single request.
func writeKeys(client *http.Client, baseURL string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	body, _ := json.Marshal(values)
	resp, err := client.PostForm(baseURL+"/datastore", url.Values{"json": {string(body)}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("POST %s/datastore: %s", baseURL, resp.Status)
	}
	return nil
}

func (d Datastore) str(key string) string {
	if v, ok := d[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func (d Datastore) bankIndex(name string) (int, bool) { return d.bankIdx("ibank", name) }

// bankIdx finds a bank by name in "ibank" (sources, inputs) or "obank" (destinations, outputs).
func (d Datastore) bankIdx(kind, name string) (int, bool) {
	for i := 0; ; i++ {
		v, ok := d[fmt.Sprintf("ext/%s/%d/name", kind, i)]
		if !ok {
			return 0, false
		}
		if v == name {
			return i, true
		}
	}
}

// channelKey turns "Analog 1" into "ext/obank/2/ch/0/<leaf>" using this device's bank names.
func (d Datastore) channelKey(kind, ref, leaf string) (string, error) {
	bank, ch, ok := channelRef(ref)
	if !ok {
		return "", fmt.Errorf("%q is not \"<bank> <channel>\"", ref)
	}
	idx, ok := d.bankIdx(kind, bank)
	if !ok {
		side := "input"
		if kind == "obank" {
			side = "output"
		}
		return "", fmt.Errorf("no %s bank named %q", side, bank)
	}
	return fmt.Sprintf("ext/%s/%d/ch/%d/%s", kind, idx, ch-1, leaf), nil
}

// sourceValue turns a source like "Mix Aux 5" into the router value "13:4".
func (d Datastore) sourceValue(ref string) (string, error) {
	key, err := d.channelKey("ibank", ref, "")
	if err != nil {
		return "", err
	}
	var bank, ch int
	_, _ = fmt.Sscanf(key, "ext/ibank/%d/ch/%d/", &bank, &ch)
	return fmt.Sprintf("%d:%d", bank, ch), nil
}

// sourceLabel turns a router value like "13:4" back into "Mix Aux 5".
func (d Datastore) sourceLabel(v string) string {
	var bank, ch int
	if _, err := fmt.Sscanf(v, "%d:%d", &bank, &ch); err != nil {
		return "(none)"
	}
	return fmt.Sprintf("%s %d", d.str(fmt.Sprintf("ext/ibank/%d/name", bank)), ch+1)
}

func channelRef(ref string) (bank string, ch int, ok bool) {
	m := bankNotation.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return "", 0, false
	}
	ch, _ = strconv.Atoi(m[2])
	return m[1], ch, ch > 0
}

func inputNamePath(bank, ch int) string {
	return fmt.Sprintf("ext/ibank/%d/ch/%d/name", bank, ch-1)
}

func (d Datastore) settings() map[string]string {
	out := map[string]string{}
	for k := range d {
		if settingKey.MatchString(k) {
			out[k] = d.str(k)
		}
	}
	return out
}

// avbEntity finds the AVB uid of the entity with the given name, as seen from this device.
func (d Datastore) avbEntity(name string) (string, bool) {
	for k, v := range d {
		if strings.HasPrefix(k, "avb/") && strings.HasSuffix(k, "/entity_name") && strings.EqualFold(fmt.Sprint(v), name) {
			return strings.Split(k, "/")[1], true
		}
	}
	return "", false
}

// streamTalker returns the "uid:index" talker feeding a listener's input stream (1-based).
func (d Datastore) streamTalker(listenerUID string, stream int) string {
	return d.str("avb/" + listenerUID + "/cfg/0/input_streams/" + strconv.Itoa(stream-1) + "/talker")
}
