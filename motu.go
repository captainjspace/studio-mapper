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

var inputNameKey = regexp.MustCompile(`^ext/ibank/\d+/ch/\d+/name$`)

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

func (d Datastore) bankIndex(name string) (int, bool) {
	for i := 0; ; i++ {
		v, ok := d[fmt.Sprintf("ext/ibank/%d/name", i)]
		if !ok {
			return 0, false
		}
		if v == name {
			return i, true
		}
	}
}

func inputNamePath(bank, ch int) string {
	return fmt.Sprintf("ext/ibank/%d/ch/%d/name", bank, ch-1)
}

func (d Datastore) inputNames() map[string]string {
	names := map[string]string{}
	for k := range d {
		if inputNameKey.MatchString(k) {
			names[k] = d.str(k)
		}
	}
	return names
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
