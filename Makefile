# Updated Go Studio Automation Makefile
BINARY_NAME=studio-map
REGISTRY ?= mozartsbutterfly.landmania.internal:32000
KUBE_CONTEXT ?= microk8s
IMAGE = $(REGISTRY)/studio-map:latest
PRESET_LIBRARY = $(HOME)/Music/Audio Music Apps
# podman's own auth only: ~/.docker/config.json has a gcloud helper that fails non-interactively
export REGISTRY_AUTH_FILE := $(HOME)/.config/containers/auth.json
INSTALL_DIR=$(HOME)/.local/bin

.PHONY: all test build install plan routing docs presets-index image push k8s-deploy publish deploy clean

# 1. Default Target: Builds the tool and sets up path links (Safe offline)
all: test build install

test:
	@echo "🧪 Running full project verification test suite..."
	go test -v ./...

build:
	@echo "🏗️  Compiling native binary: $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

# 2. Local Installation: Sets up the execution paths on your local machine
install:
	@echo "💾 Installing execution artifact into user pathway..."
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	mkdir -p $(HOME)/.config/studio-map
	ln -sf $(CURDIR)/studio_config.json $(HOME)/.config/studio-map/studio_config.json
	@echo "🏁 Local compilation complete. Call '$(BINARY_NAME)' from any terminal workspace."

# 3. Read the rack and show what would change (read-only)
plan: build
	@./$(BINARY_NAME) plan

# 4. Logic build sheet: tracks, stacks, buses, sends, presets (no rack needed)
routing: build
	@./$(BINARY_NAME) routing

# Docs page data: docs/index.html loads docs/studio-data.js (works from file://)
docs: build
	@./$(BINARY_NAME) routing --json | { printf 'window.STUDIO = '; cat; printf ';\n'; } > docs/studio-data.js
	@echo "📄 docs/studio-data.js updated: open docs/index.html"

# Preset names for machines without the Logic library (the container): file names only
presets-index:
	@cd "$(PRESET_LIBRARY)" && { find "Channel Strip Settings" -mindepth 2 -maxdepth 2 -name '*.cst'; \
		find "Plug-In Settings" -mindepth 2 -maxdepth 2 -name '*.pst'; } | LC_ALL=C sort > "$(CURDIR)/data/presets.txt"
	@echo "🎛️  data/presets.txt: $$(wc -l < data/presets.txt | tr -d ' ') presets"

# Containerized docs service on mozartsbutterfly (microk8s registry, NodePort 30180)
image:
	podman build --platform linux/amd64 -t $(IMAGE) -f Containerfile .

push:
	podman push --tls-verify=false $(IMAGE)

k8s-deploy:
	@mkdir -p k8s/files
	@cp studio_config.json data/studio-inputs.csv data/home-inputs.csv data/presets.txt k8s/files/
	kubectl --context $(KUBE_CONTEXT) apply -k k8s
	kubectl --context $(KUBE_CONTEXT) -n studio-map rollout restart deployment/studio-map
	kubectl --context $(KUBE_CONTEXT) -n studio-map rollout status deployment/studio-map --timeout=120s
	@echo "📖 http://mozartsbutterfly.landmania.internal:30180"

publish: image push k8s-deploy

# 5. Live Configuration Deployment: snapshots to state/, writes changed names, verifies
deploy: build
	@./$(BINARY_NAME) apply

clean:
	@echo "🧹 Purging transient testing output structures..."
	rm -f $(BINARY_NAME)
	rm -f test_output_document.xml
	rm -f MOTU_Studio_Labels.prochannelnames

