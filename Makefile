.PHONY: build test lint install-local uninstall-local

LOCAL_PLUGIN_DIR ?= $(HOME)/.cursor/plugins/local/trustguard

build: ## Build the trustguard-cursor hook binary into ./bin/
	@mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o bin/trustguard-cursor ./cli

test: ## Run the test suite
	go test -race ./cli/

lint: ## Vet the sources
	go vet ./cli/

# Cursor rejects a symlink whose target lives outside plugins/local, so the
# plugin has to be copied in. Re-run after every edit, then reload the window.
install-local: ## Install the plugin into ~/.cursor/plugins/local for testing
	@rm -rf "$(LOCAL_PLUGIN_DIR)"
	@mkdir -p "$(LOCAL_PLUGIN_DIR)"
	@cp -R trustguard/ "$(LOCAL_PLUGIN_DIR)/"
	@echo "installed $(LOCAL_PLUGIN_DIR) — run 'Developer: Reload Window' in Cursor"

uninstall-local: ## Remove the locally installed plugin
	@rm -rf "$(LOCAL_PLUGIN_DIR)"
	@echo "removed $(LOCAL_PLUGIN_DIR)"
