package featureflag

import (
	"testing"
)

func TestDefaults(t *testing.T) {
	if !Enabled(FlagParallelTools) {
		t.Error("PARALLEL_TOOLS should default to true")
	}
	if Enabled(FlagSkillLoader) {
		t.Error("SKILL_LOADER should default to false")
	}
	if Enabled("UNKNOWN_FLAG") {
		t.Error("unknown flag should be false")
	}
}

func TestEnvOverride_Enables(t *testing.T) {
	t.Setenv("AIEVO_FEATURE_"+FlagSkillLoader, "1")
	Reload()
	if !Enabled(FlagSkillLoader) {
		t.Error("SKILL_LOADER=1 should enable")
	}
}

func TestEnvOverride_Disables(t *testing.T) {
	t.Setenv("AIEVO_FEATURE_"+FlagParallelTools, "0")
	Reload()
	if Enabled(FlagParallelTools) {
		t.Error("PARALLEL_TOOLS=0 should disable")
	}
}

func TestEnvOverride_AcceptsMultipleFormats(t *testing.T) {
	for _, v := range []string{"true", "TRUE", "yes", "on"} {
		t.Setenv("AIEVO_FEATURE_"+FlagSkillLoader, v)
		Reload()
		if !Enabled(FlagSkillLoader) {
			t.Errorf("value %q should enable", v)
		}
	}
}

func TestSet_OverridesAtRuntime(t *testing.T) {
	t.Cleanup(Reload)
	Set(FlagParallelTools, false)
	if Enabled(FlagParallelTools) {
		t.Error("Set(false) should disable")
	}
}

func TestSnapshot(t *testing.T) {
	snap := Snapshot()
	if len(snap) == 0 {
		t.Fatal("snapshot empty")
	}
	// Mutating the snapshot must not affect the live state.
	snap[FlagParallelTools] = !snap[FlagParallelTools]
	if Enabled(FlagParallelTools) == snap[FlagParallelTools] {
		t.Error("snapshot mutation leaked into live state")
	}
}
