.PHONY: build test lint

build: ## Build the trustguard-cursor hook binary into ./bin/
	@mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o bin/trustguard-cursor ./cli

test: ## Run the test suite
	go test -race ./cli/

lint: ## Vet the sources
	go vet ./cli/
