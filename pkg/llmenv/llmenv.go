// Package llmenv builds a llm.LLM from the conventional environment variables,
// honouring AIEVO_PROVIDER / ANTHROPIC_* / OPENAI_* so example mains don't
// each need to repeat the wiring.
//
// Selection order:
//
//	1. AIEVO_PROVIDER=anthropic|openai      explicit override
//	2. ANTHROPIC_AUTH_TOKEN / API_KEY set   → anthropic
//	3. otherwise                            → openai
package llmenv

import (
	"errors"
	"os"

	"github.com/samson-samson/aievo-next/pkg/llm"
	"github.com/samson-samson/aievo-next/pkg/llm/anthropic"
	"github.com/samson-samson/aievo-next/pkg/llm/openai"
)

// Resolved bundles the chosen client with the model name to use.
type Resolved struct {
	Client llm.LLM
	Model  string
}

// FromEnv reads the env and returns a ready-to-use client + model.
func FromEnv() (Resolved, error) {
	switch os.Getenv("AIEVO_PROVIDER") {
	case "anthropic":
		return anthropicFromEnv()
	case "openai":
		return openaiFromEnv()
	}
	if tok := firstNonEmpty("ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY"); tok != "" {
		return anthropicFromEnv()
	}
	return openaiFromEnv()
}

func anthropicFromEnv() (Resolved, error) {
	tok := firstNonEmpty("ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY")
	if tok == "" {
		return Resolved{}, errors.New("ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY required")
	}
	model := firstNonEmpty("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-opus-4-5"
	}
	return Resolved{
		Client: anthropic.New(anthropic.Config{
			APIKey:  tok,
			BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
		}),
		Model: model,
	}, nil
}

func openaiFromEnv() (Resolved, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return Resolved{}, errors.New("OPENAI_API_KEY required")
	}
	model := firstNonEmpty("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return Resolved{
		Client: openai.New(openai.Config{
			APIKey:  key,
			BaseURL: os.Getenv("OPENAI_BASE_URL"),
		}),
		Model: model,
	}, nil
}

func firstNonEmpty(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
