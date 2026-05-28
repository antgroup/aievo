.PHONY: help test test-v test-race vet build install validate smoke bench tidy clean \
        examples example-engineer example-paper example-battle-auto example-battle-custom \
        example-werewolf example-undercover-auto example-undercover-custom \
        docker-build docker-shell quickstart fmt lint

# Default target — `make` shows help.
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "; printf "\033[1maievo-next\033[0m — Makefile targets\n\n"} \
	      /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ----- Core dev loops -----

test: ## Run all unit tests
	go test ./...

test-v: ## Verbose test with race detector
	go test -race -v ./...

test-race: ## Race-detector tests only (CI gate)
	go test -race ./...

vet: ## go vet across the module
	go vet ./...

fmt: ## gofmt -w everything
	gofmt -w .

lint: vet ## vet + optional staticcheck
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed (optional)"

tidy: ## go mod tidy
	go mod tidy

# ----- Build & install -----

build: ## Compile every binary into ./bin/
	@mkdir -p bin
	go build -o bin/validate ./cmd/validate
	go build -o bin/smoke    ./cmd/smoke
	@for ex in engineer paper-write battle-auto-sop battle-custom-sop werewolf undercover-auto-sop undercover-custom-sop; do \
	  echo "  → bin/$$ex"; \
	  go build -o bin/$$ex ./examples/$$ex; \
	done

install: ## Install the validate + smoke binaries into $$GOPATH/bin
	go install ./cmd/validate ./cmd/smoke

# ----- Verification -----

validate: ## Run the no-API-key architecture validation (40 checks)
	go run ./cmd/validate

smoke: ## Run a real-LLM smoke test (needs OPENAI_API_KEY or ANTHROPIC_AUTH_TOKEN)
	go run ./cmd/smoke

bench: ## Run the parallel-tool-dispatch benchmark
	go test -bench=. -benchmem -run=^$$ ./benchmark/...

# ----- One-line examples (need LLM creds) -----

examples: example-engineer example-undercover-auto ## Run engineer + undercover-auto-sop

example-engineer:           ## Run the single-agent code engineer demo
	go run ./examples/engineer
example-paper:              ## Run the 5-agent paper-writing demo
	go run ./examples/paper-write
example-battle-auto:        ## Run the 6-agent climate debate (auto SOP)
	go run ./examples/battle-auto-sop
example-battle-custom:      ## Run the 9-node 2-round debate (custom SOP)
	go run ./examples/battle-custom-sop
example-werewolf:           ## Run the 7-player werewolf game
	go run ./examples/werewolf
example-undercover-auto:    ## Run the 6-agent undercover game (auto SOP)
	go run ./examples/undercover-auto-sop
example-undercover-custom:  ## Run the 5-agent undercover game (custom SOP)
	go run ./examples/undercover-custom-sop

# ----- Container -----

docker-build: ## Build the runtime container image
	docker build -t aievo-next:latest .

docker-shell: ## Open a shell in the runtime container (mounts ~/.aievo)
	docker run --rm -it \
	  -v $$HOME/.aievo:/root/.aievo \
	  -e OPENAI_API_KEY -e OPENAI_BASE_URL -e OPENAI_MODEL \
	  -e ANTHROPIC_AUTH_TOKEN -e ANTHROPIC_BASE_URL -e ANTHROPIC_MODEL \
	  aievo-next:latest sh

# ----- Bootstrap -----

quickstart: ## One-shot: tidy + validate (works without API keys)
	@./scripts/quickstart.sh

clean: ## Remove build/test caches and bin/
	go clean -cache -testcache
	rm -rf bin/
