package llm

import (
	"testing"
)

func TestPickProvider_ModelPrefix(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4-5":  ProviderAnthropic,
		"claude-sonnet-4":  ProviderAnthropic,
		"gpt-4o-mini":      ProviderOpenAI,
		"gpt-5":            ProviderOpenAI,
		"o1-preview":       ProviderOpenAI,
		"o3-mini":          ProviderOpenAI,
		"llama3":           ProviderOpenAI, // default fallback
		"":                 ProviderOpenAI,
	}
	for model, want := range cases {
		if got := PickProvider(model); got != want {
			t.Errorf("PickProvider(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestPickProvider_EnvOverride(t *testing.T) {
	t.Setenv("AIEVO_PROVIDER", "anthropic")
	if got := PickProvider("gpt-4o"); got != "anthropic" {
		t.Errorf("env override failed: got %q", got)
	}
}
