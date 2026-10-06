package cmd

import (
	"strings"
	"testing"

	loom "github.com/orieken/loom"
	"github.com/orieken/loom/cmd/loom/internal/levels"
)

type fakeLevelProbe struct {
	installedBundles map[string]bool
	mcpAnswering     bool
	telemetryPresent bool
	policiesFound    bool
	repoIndexBuilt   bool
}

func (probe fakeLevelProbe) pathBundleInstalled(bundle levels.Bundle) (bool, string) {
	if probe.installedBundles[bundle.ID] {
		return true, "bundle \"" + bundle.ID + "\" installed"
	}
	return false, "bundle \"" + bundle.ID + "\" not fully installed"
}

func (probe fakeLevelProbe) mcpServerAnswering() (bool, string) {
	if probe.mcpAnswering {
		return true, "MCP server answering tools/list (6 tools)"
	}
	return false, "MCP server configured but not answering tools/list"
}

func (probe fakeLevelProbe) telemetryStreamPresent() (bool, string) {
	if probe.telemetryPresent {
		return true, "telemetry stream present"
	}
	return false, "telemetry stream absent"
}

func (probe fakeLevelProbe) repoIndexPresent() (bool, string) {
	if probe.repoIndexBuilt {
		return true, "repo graph present"
	}
	return false, "repo graph not built: run `loom index`"
}

func (probe fakeLevelProbe) policiesPresent() (bool, string) {
	if probe.policiesFound {
		return true, "policy files present"
	}
	return false, "no policy files"
}

// maturityTestProfile mirrors the real shared/levels.yaml shape: level 3
// carries a docs-only bundle that must never confer the level, and levels
// 2-4 carry roadmap-gated bundles.
func maturityTestProfile(landed ...string) levels.Profile {
	return levels.Profile{
		Version: 1,
		Landed:  landed,
		Levels: []levels.Level{
			{Level: 1, Name: "Foundational prompts", Bundles: []levels.Bundle{
				{ID: "core-rules", Paths: []string{"shared/rules/approval-gates.md"}},
				{ID: "agents", Paths: []string{"shared/agents"}},
			}},
			{Level: 2, Name: "Coordinated multi-agent", Bundles: []levels.Bundle{
				{ID: "mcp-registration", Requires: "D.1", Action: "mcp-config"},
				{ID: "workflows", Paths: []string{"shared/workflows"}},
				{ID: "executor", Requires: "M0.4", Action: "executor"},
			}},
			{Level: 3, Name: "Observed and governed", Bundles: []levels.Bundle{
				{ID: "telemetry-docs", DocsOnly: true, Paths: []string{"shared/telemetry"}},
				{ID: "telemetry-stream", Requires: "L3.9", Action: "telemetry"},
				{ID: "policy-engine", Requires: "L2.16", Action: "policies"},
			}},
			{Level: 4, Name: "Self-improving", Bundles: []levels.Bundle{
				{ID: "evaluation", DocsOnly: true, Paths: []string{"shared/evaluation"}},
				{ID: "eval-workspace", Requires: "L4.1", Paths: []string{"shared/eval-workspace"}},
			}},
		},
	}
}

func allBundlesInstalled() map[string]bool {
	return map[string]bool{
		"core-rules": true, "agents": true, "workflows": true,
		"telemetry-docs": true, "evaluation": true, "eval-workspace": true,
	}
}

func TestInferMaturityAcrossAllLevels(t *testing.T) {
	cases := []struct {
		name            string
		landed          []string
		probe           fakeLevelProbe
		wantLevel       int
		wantNextLevel   int
		wantUnreachable bool
		wantGaps        []string
	}{
		{
			name:          "nothing installed reports below level 1",
			landed:        []string{"D.1"},
			probe:         fakeLevelProbe{installedBundles: map[string]bool{}},
			wantLevel:     0,
			wantNextLevel: 1,
			wantGaps:      []string{`bundle "core-rules" not fully installed`, `bundle "agents" not fully installed`},
		},
		{
			name:          "level 2 docs present but server dead is still level 1",
			landed:        []string{"D.1"},
			probe:         fakeLevelProbe{installedBundles: allBundlesInstalled()},
			wantLevel:     1,
			wantNextLevel: 2,
			wantGaps: []string{
				"MCP server configured but not answering tools/list",
				"executor: gated on roadmap item M0.4 (not landed)",
			},
		},
		{
			name:            "level 2 attained and fully gated level 3 is unreachable",
			landed:          []string{"D.1"},
			probe:           fakeLevelProbe{installedBundles: allBundlesInstalled(), mcpAnswering: true},
			wantLevel:       2,
			wantNextLevel:   3,
			wantUnreachable: true,
			wantGaps: []string{
				"telemetry-stream: gated on roadmap item L3.9 (not landed)",
				"policy-engine: gated on roadmap item L2.16 (not landed)",
			},
		},
		{
			name:   "docs-only telemetry bundle never confers level 3",
			landed: []string{"D.1", "M0.4", "L3.9", "L2.16"},
			probe: fakeLevelProbe{
				installedBundles: allBundlesInstalled(), mcpAnswering: true, policiesFound: true,
			},
			wantLevel:     1,
			wantNextLevel: 2,
			wantGaps:      []string{`executor: no health check exists for action "executor" yet`},
		},
		{
			name:   "level 3 attained when telemetry and policies answer",
			landed: []string{"D.1", "L3.9", "L2.16"},
			probe: fakeLevelProbe{
				installedBundles: allBundlesInstalled(), mcpAnswering: true,
				telemetryPresent: true, policiesFound: true,
			},
			wantLevel:       3,
			wantNextLevel:   4,
			wantUnreachable: true,
			wantGaps:        []string{"eval-workspace: gated on roadmap item L4.1 (not landed)"},
		},
		{
			name:   "level 4 attained leaves no gap checklist",
			landed: []string{"D.1", "L3.9", "L2.16", "L4.1"},
			probe: fakeLevelProbe{
				installedBundles: allBundlesInstalled(), mcpAnswering: true,
				telemetryPresent: true, policiesFound: true,
			},
			wantLevel:     4,
			wantNextLevel: 0,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			profile := maturityTestProfile(testCase.landed...)
			report := inferMaturity(assessLevels(profile, testCase.probe))
			assertMaturityReport(t, report, testCase.wantLevel, testCase.wantNextLevel, testCase.wantUnreachable, testCase.wantGaps)
		})
	}
}

func assertMaturityReport(t *testing.T, report maturityReport, wantLevel, wantNextLevel int, wantUnreachable bool, wantGaps []string) {
	t.Helper()
	if report.level != wantLevel {
		t.Fatalf("level = %d, want %d (gaps: %v)", report.level, wantLevel, report.gaps)
	}
	if report.nextLevel != wantNextLevel {
		t.Fatalf("nextLevel = %d, want %d", report.nextLevel, wantNextLevel)
	}
	if report.nextIsUnreachable != wantUnreachable {
		t.Fatalf("nextIsUnreachable = %v, want %v", report.nextIsUnreachable, wantUnreachable)
	}
	assertGapsContain(t, report.gaps, wantGaps)
}

func assertGapsContain(t *testing.T, gaps, wanted []string) {
	t.Helper()
	joined := strings.Join(gaps, "\n")
	for _, want := range wanted {
		if !strings.Contains(joined, want) {
			t.Errorf("gaps %v missing %q", gaps, want)
		}
	}
}

func repoIndexProfile() levels.Profile {
	return levels.Profile{Version: 1, Landed: []string{"L3.63"}, Levels: []levels.Level{
		{Level: 1, Name: "L1", Bundles: []levels.Bundle{{ID: "agents", Paths: []string{"shared/agents"}}}},
		{Level: 2, Name: "L2", Bundles: []levels.Bundle{
			{ID: "repo-index", Requires: "L3.63", Action: "repo-index"},
			{ID: "repo-docs", Requires: "L3.63", OptIn: true, Action: "repo-docs"},
		}},
		{Level: 3, Name: "L3", Bundles: []levels.Bundle{{ID: "later", Requires: "L9.9"}}},
		{Level: 4, Name: "L4", Bundles: []levels.Bundle{{ID: "later", Requires: "L9.9"}}},
	}}
}

// Level 2 needs the repo graph: without it the MCP repo tools answer "run loom index".
func TestLevelTwoNeedsTheRepoGraphToExist(t *testing.T) {
	without := assessLevels(repoIndexProfile(), fakeLevelProbe{installedBundles: map[string]bool{"agents": true}})
	if without[1].isAttained() {
		t.Error("a project that never ran `loom index` has not attained level 2")
	}
	report := inferMaturity(without)
	if report.level != 1 || len(report.gaps) != 1 || !strings.Contains(report.gaps[0], "loom index") {
		t.Errorf("the gap must say what to run: %+v", report)
	}
	with := assessLevels(repoIndexProfile(), fakeLevelProbe{installedBundles: map[string]bool{"agents": true}, repoIndexBuilt: true})
	if !with[1].isAttained() {
		t.Errorf("%+v", with[1])
	}
}

// An opt-in bundle is a choice. Declining it must never lower a level, and
// adopting it must never raise one.
func TestAnOptInBundleIsNeverLevelEvidence(t *testing.T) {
	assessments := assessLevels(repoIndexProfile(), fakeLevelProbe{installedBundles: map[string]bool{"agents": true}, repoIndexBuilt: true})
	if len(assessments[1].evidence) != 1 {
		t.Errorf("only repo-index is evidence at level 2: %+v", assessments[1].evidence)
	}
	for _, item := range assessments[1].evidence {
		if strings.Contains(item.detail, "repo-docs") {
			t.Errorf("repo-docs leaked into the evidence: %+v", item)
		}
	}
}

func TestTheRealProbeLooksForTheGraphUnderLoom(t *testing.T) {
	target := t.TempDir()
	probe := healthLevelProbe{target: target}
	if ok, detail := probe.repoIndexPresent(); ok || !strings.Contains(detail, "run `loom index`") {
		t.Errorf("%v %q", ok, detail)
	}
	writeRepoFile(t, target, ".loom/graph.json", "{}")
	if ok, detail := probe.repoIndexPresent(); !ok || !strings.Contains(detail, ".loom") {
		t.Errorf("%v %q", ok, detail)
	}
}

func shippedLevelTwoBundles(t *testing.T) (levels.Profile, map[string]levels.Bundle) {
	t.Helper()
	profile, err := levels.Load(loom.FrameworkFS)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]levels.Bundle{}
	for _, bundle := range profile.Levels[1].Bundles {
		found[bundle.ID] = bundle
	}
	return profile, found
}

func TestTheShippedProfileMakesRepoIndexLevelTwoEvidence(t *testing.T) {
	profile, found := shippedLevelTwoBundles(t)
	index := found["repo-index"]
	if index.Action != "repo-index" || index.OptIn || index.Requires != "L3.63" || !profile.IsLanded("L3.63") {
		t.Errorf("%+v", index)
	}
}

func TestTheShippedProfileMakesRepoDocsOptIn(t *testing.T) {
	_, found := shippedLevelTwoBundles(t)
	if docs := found["repo-docs"]; !docs.OptIn || docs.Action != "repo-docs" {
		t.Errorf("%+v", docs)
	}
}
