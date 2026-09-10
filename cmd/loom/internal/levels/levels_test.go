package levels_test

import (
	"testing"

	loom "github.com/orieken/loom"
	"github.com/orieken/loom/cmd/loom/internal/levels"
)

func loadProfile(t *testing.T) levels.Profile {
	t.Helper()
	profile, err := levels.Load(loom.FrameworkFS)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	return profile
}

func TestLoadParsesFourOrderedLevels(t *testing.T) {
	profile := loadProfile(t)
	if len(profile.Levels) != levels.MaxLevel {
		t.Fatalf("got %d levels, want %d", len(profile.Levels), levels.MaxLevel)
	}
	for index, level := range profile.Levels {
		if level.Level != index+1 {
			t.Errorf("levels[%d].Level = %d, want %d", index, level.Level, index+1)
		}
		if level.Name == "" {
			t.Errorf("level %d has no name", level.Level)
		}
	}
}

func TestSelectIsCumulativeAndBounded(t *testing.T) {
	profile := loadProfile(t)
	tests := []struct {
		target    int
		wantCount int
		wantErr   bool
	}{
		{0, 0, true},
		{1, 1, false},
		{2, 2, false},
		{4, 4, false},
		{5, 0, true},
	}
	for _, tt := range tests {
		selected, err := profile.Select(tt.target)
		if (err != nil) != tt.wantErr {
			t.Errorf("Select(%d) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			continue
		}
		if len(selected) != tt.wantCount {
			t.Errorf("Select(%d) returned %d levels, want %d", tt.target, len(selected), tt.wantCount)
		}
	}
}

func TestCoreRuleFilesExistInEmbeddedContent(t *testing.T) {
	profile := loadProfile(t)
	names, err := profile.CoreRuleNames()
	if err != nil {
		t.Fatalf("CoreRuleNames() failed: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no core rules declared")
	}
	for _, name := range names {
		if _, err := loom.FrameworkFS.ReadFile("shared/rules/" + name); err != nil {
			t.Errorf("core rule %s not readable from embedded content: %v", name, err)
		}
	}
}

func TestOnDemandModulesResolveAndExist(t *testing.T) {
	profile := loadProfile(t)
	for _, module := range profile.Levels[0].OnDemandModules {
		name := profile.OnDemandRuleName(module.Stack)
		if name == "" {
			t.Errorf("stack %q resolves to no rule", module.Stack)
			continue
		}
		if _, err := loom.FrameworkFS.ReadFile("shared/rules/" + name); err != nil {
			t.Errorf("on-demand rule %s not readable from embedded content: %v", name, err)
		}
	}
	if got := profile.OnDemandRuleName("cobol"); got != "" {
		t.Errorf("unknown stack resolved to %q, want empty", got)
	}
}

// TestLandedGatesMatchProfile guards the discipline shared/levels.yaml asks
// for in its own header: update `landed` in the commit that ships an item.
//
// This test previously asserted M0.4 was NOT landed, and kept asserting it
// for the nine days after M0.4 shipped — so install told users the executor
// had not shipped when it had (roadmap L3.26). A landed item and an unlanded
// one are both checked now, because pinning only one direction is what let
// the list drift.
func TestLandedGatesMatchProfile(t *testing.T) {
	profile := loadProfile(t)
	for _, shipped := range []string{"D.1", "M0.4", "L3.9", "L2.16"} {
		if !profile.IsLanded(shipped) {
			t.Errorf("%s has shipped and must be marked landed", shipped)
		}
	}
	if profile.IsLanded("L4.1") {
		t.Error("L4.1 (bounded Reflexion) has not shipped and must not be marked landed")
	}
}

// TestCoreRulesBundleStaysUnderCeiling is the D.3 context-budget fitness
// function: the level 1 core rules bundle must stay under the ceiling
// documented in shared/levels.yaml. If this fails, either trim the core
// rules or raise the ceiling deliberately in the same commit.
func TestCoreRulesBundleStaysUnderCeiling(t *testing.T) {
	profile := loadProfile(t)
	ceiling := profile.ContextBudget.CoreRulesCeilingBytes
	if ceiling <= 0 {
		t.Fatal("contextBudget.coreRulesCeilingBytes is not declared")
	}
	names, err := profile.CoreRuleNames()
	if err != nil {
		t.Fatalf("CoreRuleNames() failed: %v", err)
	}
	total := 0
	for _, name := range names {
		content, readErr := loom.FrameworkFS.ReadFile("shared/rules/" + name)
		if readErr != nil {
			t.Fatalf("read core rule %s: %v", name, readErr)
		}
		total += len(content)
	}
	if total > ceiling {
		t.Errorf("core rules bundle is %d bytes, over the %d-byte ceiling in %s", total, ceiling, levels.ProfilePath)
	}
}
