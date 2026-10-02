package main

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
)

//go:embed docs/index.html
var indexHTML []byte

// runServe serves the docs page and the routing JSON, regenerated from the sheet and config on every request.
func runServe(args []string) error {
	addr, _ := takeFlag(args, "addr")
	addr = cmp.Or(addr, ":8080")
	fmt.Printf("studio-map serving on %s (config %s)\n", addr, env.paths.Config)
	return http.ListenAndServe(addr, newServer(env.paths))
}

func newServer(p Paths) http.Handler {
	// rigConfig re-reads the config and selects ?rig= (default: the config's default rig)
	rigConfig := func(r *http.Request) (Paths, Config, error) {
		cfg, err := loadConfig(p.Config)
		if err != nil {
			return p, cfg, err
		}
		cfg, err = cfg.withRig(cmp.Or(r.URL.Query().Get("rig"), cfg.defaultRig()))
		rp := p
		rp.Sheet = p.abs(cfg.Sheet)
		return rp, cfg, err
	}
	routing := func(r *http.Request) (any, error) {
		rp, cfg, err := rigConfig(r)
		if err != nil {
			return nil, err
		}
		g, presets, mode, problems, err := buildRouting(rp, cfg, r.URL.Query().Has("stems"))
		if err != nil {
			return nil, err
		}
		return routingDoc(cfg.RigName, g, presets, mode, problems), nil
	}
	inputs := func(r *http.Request) (any, error) {
		rp, cfg, err := rigConfig(r)
		if err != nil {
			return nil, err
		}
		in, err := LoadSheet(rp.Sheet, cfg)
		if err != nil {
			return nil, err
		}
		return inputsDoc(cfg.RigName, in, presetsFor(rp, cfg)), nil
	}
	rigs := func(r *http.Request) (any, error) {
		cfg, err := loadConfig(p.Config)
		return map[string]any{"default": cfg.defaultRig(), "rigs": sortedKeys(cfg.Rigs)}, err
	}
	// serve renders a fresh document per request; wrap adapts it (e.g. into a script for the page)
	serve := func(contentType string, wrap func([]byte) []byte, doc func(*http.Request) (any, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			d, err := doc(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			body, err := json.MarshalIndent(d, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(wrap(body))
		}
	}
	asIs := func(b []byte) []byte { return b }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/routing", serve("application/json", asIs, routing))
	mux.HandleFunc("GET /api/inputs", serve("application/json", asIs, inputs))
	mux.HandleFunc("GET /api/rigs", serve("application/json", asIs, rigs))
	mux.HandleFunc("GET /studio-data.js", serve("text/javascript", func(b []byte) []byte {
		return append(append([]byte("window.STUDIO = "), b...), ";\n"...)
	}, routing))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	return mux
}
