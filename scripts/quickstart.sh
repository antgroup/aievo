#!/usr/bin/env bash
# quickstart.sh — first-run bootstrap. Safe to re-run.
#
#   1. checks Go is installed and version meets minimum
#   2. runs `go mod tidy`
#   3. runs the no-API-key architecture validation
#   4. prints next steps (running examples with credentials)
set -euo pipefail

MIN_GO_MAJOR=1
MIN_GO_MINOR=23

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
dim()  { printf "\033[2m%s\033[0m\n" "$*"; }
ok()   { printf "  \033[32m✓\033[0m %s\n" "$*"; }
warn() { printf "  \033[33m!\033[0m %s\n" "$*"; }
err()  { printf "  \033[31m✗\033[0m %s\n" "$*"; }

bold "aievo-next quickstart"
echo

# ----- 1. Go version -----
echo "[1/4] check Go toolchain"
if ! command -v go >/dev/null 2>&1; then
  err "Go is not installed. Get it from https://go.dev/dl/ (>= ${MIN_GO_MAJOR}.${MIN_GO_MINOR})"
  exit 1
fi
gover=$(go version | sed -nE 's/.*go([0-9]+)\.([0-9]+).*/\1.\2/p')
gomaj=$(echo "$gover" | cut -d. -f1)
gomin=$(echo "$gover" | cut -d. -f2)
if [ "$gomaj" -lt "$MIN_GO_MAJOR" ] || { [ "$gomaj" -eq "$MIN_GO_MAJOR" ] && [ "$gomin" -lt "$MIN_GO_MINOR" ]; }; then
  err "Go $gover is below the minimum ${MIN_GO_MAJOR}.${MIN_GO_MINOR}"
  exit 1
fi
ok "Go $gover ≥ ${MIN_GO_MAJOR}.${MIN_GO_MINOR}"

# ----- 2. mod tidy -----
echo "[2/4] resolve dependencies"
go mod tidy
ok "go mod tidy"

# ----- 3. architecture validation -----
echo "[3/4] architecture validation (no API key needed)"
go run ./cmd/validate
echo

# ----- 4. credentials hint -----
echo "[4/4] credentials check"
if [ -n "${ANTHROPIC_AUTH_TOKEN:-${ANTHROPIC_API_KEY:-}}" ]; then
  ok "Anthropic credentials detected → make smoke"
elif [ -n "${OPENAI_API_KEY:-}" ]; then
  ok "OpenAI credentials detected → make smoke"
else
  warn "No LLM credentials in env."
  dim "    Set one of:"
  dim "      export OPENAI_API_KEY=sk-..."
  dim "      export ANTHROPIC_AUTH_TOKEN=..."
  dim "    Or copy .env.example to .env and 'source .env'."
fi

echo
bold "Ready. Try:"
echo "    make help            # show all targets"
echo "    make validate        # re-run the architecture check"
echo "    make smoke           # hit your LLM with a 5-word prompt"
echo "    make example-engineer  # single-agent code generation demo"
