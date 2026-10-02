// Package server serves the docs page and the routing/inputs JSON, regenerated from the sheet and config on every request.
package server

import (
	"cmp"
	"encoding/json"
	"net/http"

	"studio/engine/docs"
	"studio/engine/internal/config"
	"studio/engine/internal/export"
	"studio/engine/internal/mix"
	"studio/engine/internal/presets"
	"studio/engine/internal/sheet"
)

// New returns the HTTP handler. The server never auto-detects a rig; ?rig= picks one (default: the config's default).
func New(p config.Paths) http.Handler {
	// rigConfig re-reads the config and selects ?rig= (default: the config's default rig)
	rigConfig := func(r *http.Request) (config.Paths, config.Config, error) {
		cfg, err := config.Load(p.Config)
		if err != nil {
			return p, cfg, err
		}
		cfg, err = cfg.WithRig(cmp.Or(r.URL.Query().Get("rig"), cfg.DefaultRigName()))
		rp := p
		rp.Sheet = p.Abs(cfg.Sheet)
		return rp, cfg, err
	}
	routing := func(r *http.Request) (any, error) {
		rp, cfg, err := rigConfig(r)
		if err != nil {
			return nil, err
		}
		g, lib, mode, problems, err := mix.BuildRouting(rp, cfg, r.URL.Query().Has("stems"))
		if err != nil {
			return nil, err
		}
		return export.Routing(cfg.RigName, g, lib, mode, problems), nil
	}
	inputs := func(r *http.Request) (any, error) {
		rp, cfg, err := rigConfig(r)
		if err != nil {
			return nil, err
		}
		in, err := sheet.Load(rp.Sheet, cfg)
		if err != nil {
			return nil, err
		}
		return export.Inputs(cfg.RigName, in, presets.For(rp, cfg)), nil
	}
	rigs := func(r *http.Request) (any, error) {
		cfg, err := config.Load(p.Config)
		return map[string]any{"default": cfg.DefaultRigName(), "rigs": cfg.RigNames()}, err
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
		_, _ = w.Write(docs.IndexHTML)
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
