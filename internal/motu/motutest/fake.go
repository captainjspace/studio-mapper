// Package motutest provides a fake MOTU datastore device for tests.
package motutest

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"studio/engine/internal/motu"
)

// Fake serves a MOTU AVB datastore like the hardware: GET /datastore returns every key,
// GET /datastore/<key> returns {"value": …}, and POST json={...} sets keys.
type Fake struct {
	mu sync.Mutex
	DS motu.Datastore
}

func (f *Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if key, ok := strings.CutPrefix(r.URL.Path, "/datastore/"); ok && r.Method == http.MethodGet {
		v, found := f.DS[key]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": v})
		return
	}
	if r.URL.Path != "/datastore" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		var set map[string]string
		if err := json.Unmarshal([]byte(r.FormValue("json")), &set); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for k, v := range set {
			f.DS[k] = v
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(w).Encode(f.DS)
}

// Get reads a key under the lock (tests inspect state while the server runs).
func (f *Fake) Get(key string) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.DS[key]
}
