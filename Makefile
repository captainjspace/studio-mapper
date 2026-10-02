# Updated Go Studio Automation Makefile
BINARY_NAME=studio-map
INSTALL_DIR=$(HOME)/.local/bin

.PHONY: all test build install plan routing deploy clean

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

# 5. Live Configuration Deployment: snapshots to state/, writes changed names, verifies
deploy: build
	@./$(BINARY_NAME) apply

clean:
	@echo "🧹 Purging transient testing output structures..."
	rm -f $(BINARY_NAME)
	rm -f test_output_document.xml
	rm -f MOTU_Studio_Labels.prochannelnames

