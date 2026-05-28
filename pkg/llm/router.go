// Provider router: pick an LLM provider from model name or explicit env hint.
//
// Hierarchy (highest wins):
//
//	1. Env override:   AIEVO_PROVIDER=anthropic|openai|mock
//	2. Model prefix:   "claude-*" → anthropic; "gpt-*" / "o*" → openai
//	3. Fallback:       openai (cheap default; works with Ollama/DeepSeek)
//
// Designed to remove the if-elif chain Claude Code carries in
// services/api/claude.ts queryModel:1334-1372 from caller code.
package llm

import (
	"os"
	"strings"
)

// Provider names. Kept as untyped strings so external adapters can register
// without changing this package.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderMock      = "mock"
)

// PickProvider chooses a provider for `model` honouring the env hint.
func PickProvider(model string) string {
	if env := strings.ToLower(os.Getenv("AIEVO_PROVIDER")); env != "" {
		return env
	}
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return ProviderAnthropic
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
		return ProviderOpenAI
	}
	return ProviderOpenAI
}
