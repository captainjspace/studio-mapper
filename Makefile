# Updated Go Studio Automation Makefile
BINARY_NAME=studio-map
INSTALL_DIR=$(HOME)/.local/bin
LOGIC_DIR=$(HOME)/Library/Preferences/Logic

.PHONY: all test test-harness build install deploy clean

# 1. Default Target: Builds the tool and sets up path links (Safe offline)
all: test test-harness build install

test:
	@echo "🧪 Running full project verification test suite..."
	go test -v ./...

test-harness:
	@echo "📦 Validating XML template hydration isolation rules..."
	go test -v -run TestDeclarativeTemplateHydration

build:
	@echo "🏗️  Compiling native binary: $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

# 2. Local Installation: Sets up the execution paths on your local machine
install:
	@echo "💾 Installing execution artifact into user pathway..."
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "🏁 Local compilation complete. Call '$(BINARY_NAME)' from any terminal workspace."

# 3. Live Configuration Deployment: Pushes changes to the rack (Fails if network is down)
deploy: build
	@echo "🔍 Scanning studio network endpoints for dynamic hardware profiling..."
	@mkdir -p $(LOGIC_DIR)
	@./$(BINARY_NAME) || (echo "❌ Deployment aborted: Physical network hardware is unreachable." && exit 1)
	@if [ -f "MOTU_Studio_Labels.prochannelnames" ]; then \
		cp MOTU_Studio_Labels.prochannelnames $(LOGIC_DIR)/MOTU_Studio_Labels.prochannelnames; \
		echo "🍎 Logic Pro mapping metadata updated successfully."; \
		fi
	@echo "🎉 Complete live deployment successful."

clean:
	@echo "🧹 Purging transient testing output structures..."
	rm -f $(BINARY_NAME)
	rm -f test_output_document.xml
	rm -f MOTU_Studio_Labels.prochannelnames

