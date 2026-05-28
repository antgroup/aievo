// Package featureflag implements an env-var-driven feature flag system.
//
// Why feature flags (borrowed from Claude Code's bun:bundle feature())?
//   - Allow experimental / heavy / API-key-gated functionality to ship
//     in the same binary as stable code without paying runtime cost.
//   - One source of truth in init() — env vars override defaults.
//   - Go doesn't have dead-code elimination based on constants, but a
//     map lookup is ~10ns and never goroutine-contended (read-only after init).
//
// Naming convention: env var = AIEVO_FEATURE_<NAME>, values "1"/"true" enable,
// "0"/"false" disable. Unknown values keep the default.
package featureflag

import (
	"os"
	"strings"
	"sync"
)

const (
	// FlagParallelTools enables IsConcurrencySafe-based parallel tool dispatch.
	// Disabling reverts to aievo's original strictly-sequential behaviour.
	FlagParallelTools = "PARALLEL_TOOLS"

	// FlagAgentRetry enables the retry policy in pkg/agent on transient errors.
	FlagAgentRetry = "AGENT_RETRY"

	// FlagPersistentMemory enables a pluggable Memory backend (file/SQLite).
	// Default off (experimental). aievo's behaviour is in-memory only.
	FlagPersistentMemory = "PERSISTENT_MEMORY"

	// FlagSkillLoader enables Markdown SKILL.md autoloading at agent boot.
	// Default off (planned for v0.2).
	FlagSkillLoader = "SKILL_LOADER"

	// FlagStrictTransitions makes invalid state transitions panic instead of
	// returning an error. Useful in CI; default off in production.
	FlagStrictTransitions = "STRICT_TRANSITIONS"
)

var (
	mu       sync.RWMutex
	enabled  map[string]bool
	defaults = map[string]bool{
		FlagParallelTools:     true,
		FlagAgentRetry:        true,
		FlagPersistentMemory:  false,
		FlagSkillLoader:       false,
		FlagStrictTransitions: false,
	}
)

func init() { Reload() }

// Reload re-reads env vars and rebuilds the enabled set. Useful in tests.
func Reload() {
	mu.Lock()
	defer mu.Unlock()
	m := make(map[string]bool, len(defaults))
	for k, v := range defaults {
		m[k] = v
		switch strings.ToLower(os.Getenv("AIEVO_FEATURE_" + k)) {
		case "1", "true", "yes", "on":
			m[k] = true
		case "0", "false", "no", "off":
			m[k] = false
		}
	}
	enabled = m
}

// Enabled reports whether the named flag is on. Unknown flag → false.
func Enabled(name string) bool {
	mu.RLock()
	v := enabled[name]
	mu.RUnlock()
	return v
}

// Set overrides a flag programmatically. Intended for tests only.
func Set(name string, v bool) {
	mu.Lock()
	enabled[name] = v
	mu.Unlock()
}

// Snapshot returns a copy of the current flag set. Intended for diagnostics.
func Snapshot() map[string]bool {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]bool, len(enabled))
	for k, v := range enabled {
		out[k] = v
	}
	return out
}
