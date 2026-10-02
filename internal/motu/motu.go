// Package motu talks to a MOTU AVB device's datastore: the flat key/value tree it serves at /datastore.
package motu

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Datastore is the flat key/value tree a MOTU AVB device serves at /datastore.
type Datastore map[string]any

// settingKey matches what apply can change and restore puts back: channel names and router sources.
var settingKey = regexp.MustCompile(`^ext/[io]bank/\d+/ch/\d+/(name|src)$`)

// BankNotation is the generic "<Bank> N" form, using the device's own bank names: "Analog 1", "Mix Aux 5".
var BankNotation = regexp.MustCompile(`^([A-Za-z][A-Za-z/ -]*?)\s+(\d+)$`)

// NewClient avoids keep-alive: MOTU devices send a stray CRLF after 204 replies.
func NewClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
}

func Fetch(client *http.Client, baseURL string) (Datastore, error) {
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

// WriteKeys sets several datastore keys in a single request.
func WriteKeys(client *http.Client, baseURL string, values map[string]string) error {
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

// VerifyWrites re-reads the device and fails if any written key didn't stick.
func VerifyWrites(client *http.Client, baseURL string, writes map[string]string) error {
	ds, err := Fetch(client, baseURL)
	if err != nil {
		return err
	}
	if diff := ds.Changed(writes); len(diff) > 0 {
		return fmt.Errorf("%d names did not stick: %v", len(diff), slices.Sorted(maps.Keys(diff)))
	}
	return nil
}

func (d Datastore) Str(key string) string {
	if v, ok := d[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func (d Datastore) BankIndex(name string) (int, bool) { return d.BankIdx("ibank", name) }

// BankIdx finds a bank by name in "ibank" (sources, inputs) or "obank" (destinations, outputs).
func (d Datastore) BankIdx(kind, name string) (int, bool) {
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

// ChannelKey turns "Analog 1" into "ext/obank/2/ch/0/<leaf>" using this device's bank names.
func (d Datastore) ChannelKey(kind, ref, leaf string) (string, error) {
	bank, ch, ok := ChannelRef(ref)
	if !ok {
		return "", fmt.Errorf("%q is not \"<bank> <channel>\"", ref)
	}
	idx, ok := d.BankIdx(kind, bank)
	if !ok {
		side := "input"
		if kind == "obank" {
			side = "output"
		}
		return "", fmt.Errorf("no %s bank named %q", side, bank)
	}
	return fmt.Sprintf("ext/%s/%d/ch/%d/%s", kind, idx, ch-1, leaf), nil
}

// SourceValue turns a source like "Mix Aux 5" into the router value "13:4".
func (d Datastore) SourceValue(ref string) (string, error) {
	key, err := d.ChannelKey("ibank", ref, "")
	if err != nil {
		return "", err
	}
	var bank, ch int
	_, _ = fmt.Sscanf(key, "ext/ibank/%d/ch/%d/", &bank, &ch)
	return fmt.Sprintf("%d:%d", bank, ch), nil
}

// SourceLabel turns a router value like "13:4" back into "Mix Aux 5".
func (d Datastore) SourceLabel(v string) string {
	var bank, ch int
	if _, err := fmt.Sscanf(v, "%d:%d", &bank, &ch); err != nil {
		return "(none)"
	}
	return fmt.Sprintf("%s %d", d.Str(fmt.Sprintf("ext/ibank/%d/name", bank)), ch+1)
}

// ChannelRef splits "Mix Aux 5" into bank "Mix Aux" and channel 5.
func ChannelRef(ref string) (bank string, ch int, ok bool) {
	m := BankNotation.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return "", 0, false
	}
	ch, _ = strconv.Atoi(m[2])
	return m[1], ch, ch > 0
}

func InputNamePath(bank, ch int) string {
	return fmt.Sprintf("ext/ibank/%d/ch/%d/name", bank, ch-1)
}

// Settings returns every channel name and router source, by datastore key (what snapshots keep).
func (d Datastore) Settings() map[string]string {
	out := map[string]string{}
	for k := range d {
		if settingKey.MatchString(k) {
			out[k] = d.Str(k)
		}
	}
	return out
}

// Changed returns the entries of want that differ from this datastore.
func (d Datastore) Changed(want map[string]string) map[string]string {
	diff := map[string]string{}
	for k, v := range want {
		if d.Str(k) != v {
			diff[k] = v
		}
	}
	return diff
}

// AVBEntity finds the AVB uid of the entity with the given name, as seen from this device.
func (d Datastore) AVBEntity(name string) (string, bool) {
	for k, v := range d {
		if strings.HasPrefix(k, "avb/") && strings.HasSuffix(k, "/entity_name") && strings.EqualFold(fmt.Sprint(v), name) {
			return strings.Split(k, "/")[1], true
		}
	}
	return "", false
}

// StreamTalker returns the "uid:index" talker feeding a listener's input stream (1-based).
func (d Datastore) StreamTalker(listenerUID string, stream int) string {
	return d.Str("avb/" + listenerUID + "/cfg/0/input_streams/" + strconv.Itoa(stream-1) + "/talker")
}
