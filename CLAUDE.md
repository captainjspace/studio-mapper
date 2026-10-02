# Project Overview
- **Name:** logic-manager, studio-map
- **Summary:** Declarative studio control plane. Each location ("rig") has a device map in `studio_config.json` and a sheet (`data/*-inputs.csv`, exported from the Google Sheet). `studio-map` queries the MOTU AVB devices (datastore HTTP API), compares them with what's declared (input/output names, router routes), and applies the differences with snapshots and restore. It also renders the Logic build sheet (stacks, buses, sends, presets) and serves docs + JSON for the band app.
- **Rigs:** `oakland` (16A + 24Ai → AVB → 10pre; all real tracking) and `home` (UltraLite AVB; overdubs/mixing, UA 2-1176 on Line Out/In 1/2). Auto-detected; `--rig` overrides.

## Build & Test Commands
- **Install:** `make install` (binary to `~/.local/bin`, links `~/.config/studio-map/studio_config.json` → repo config)
- **Run:** `studio-map plan` (read-only) · `studio-map apply` · `studio-map restore <snapshot>` · `studio-map routing [--stems] [--json]` · `studio-map inputs` · `studio-map serve`
- **Test:** `make test` (`go test ./...`)
- **Lint:** `go vet ./...` and `gofmt -l .` (must print nothing)
- **Container / cluster:** `make image push k8s-deploy` (podman → microk8s registry on mozartsbutterfly, NodePort 30180)

## Code Style & Guidelines
- **Language/Stack:** go, json
- **Conventions:**
  - Keep inline comments sparse; write self-explanatory code.
  - Always run tests and linter before declaring a task complete.
  - Fail gracefully when a device is not connected (warn and skip, never crash).
  - `plan` stays read-only. Anything that writes to hardware goes through `apply` and takes a snapshot first.
  - Only declared settings are managed; everything else on a device is left alone.

## Directory Structure
- `cmd/studio-map/` - CLI: flags, rig selection, thin command wrappers
- `internal/config/` - config schema, rigs, path resolution, rig detection
- `internal/motu/` - MOTU datastore client (`motutest/`: fake device for tests)
- `internal/coreaudio/` - host input names via CoreAudio (cgo; stub off macOS)
- `internal/sheet/` - studio sheet CSV → inputs
- `internal/presets/` - Logic `.cst`/`.pst` lookup, library or `data/presets.txt` index
- `internal/mix/` - signal-flow graph and build sheet rendering (`mixtest/`: fixtures)
- `internal/export/` - versioned routing/inputs JSON (contract for docs page and band app)
- `internal/plan/` - plan, read live, apply, restore, snapshots
- `internal/server/` - docs page + JSON API
- `data/` - sheets per rig, preset index · `docs/` - docs page (embedded) · `k8s/` - kustomize · `script/` - zsh helpers · `templates/` - Logic template archives (`.tar.br`)

## Workflow Rules
- Enter plan mode for any non-trivial task (3+ steps or architectural choices).
- If an error occurs, stop and re-plan immediately rather than guessing fixes.
