package docchange

import (
	"strings"
	"testing"
)

func TestHumanTextOutsideTheMarkersSurvivesApplyAndRegeneration(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	const intro = "# lib\n\nHand-written intro that must survive.\n"
	writeFile(t, root, "lib/README.md", intro)
	approveAndApply(t, root, plan(t, root, Options{InTreeReadmes: true}).ID)
	first := readFile(t, root, "lib/README.md")
	if !strings.HasPrefix(first, intro) || !strings.Contains(first, "does lib") {
		t.Fatalf("the human's text stays and a block is appended:\n%s", first)
	}

	footer := "\nA footer a person wrote after generation.\n"
	writeFile(t, root, "lib/README.md", first+footer)
	writeFile(t, root, "lib/lib.go", "package lib\n\n// Do does something else.\nfunc Do() {}\n")
	git(t, root, "add", "-A")
	documentedRun(t, root, func(d *stateDoc) {
		d.Purpose = "regenerated " + d.Path
		d.Evidence[0].Claim = d.Purpose
	})
	approveAndApply(t, root, plan(t, root, Options{InTreeReadmes: true}).ID)
	second := readFile(t, root, "lib/README.md")
	if !strings.HasPrefix(second, intro) || !strings.HasSuffix(second, footer) {
		t.Errorf("text outside the markers changed:\n%s", second)
	}
	if !strings.Contains(second, "regenerated lib") || strings.Contains(second, "does lib") {
		t.Errorf("only the block should have been regenerated:\n%s", second)
	}
}

// handEditedReadme sets up a README whose generated block a person has edited,
// then plans a regeneration over it.
func handEditedReadme(t *testing.T) (root, edited string, change Change) {
	t.Helper()
	root = newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{InTreeReadmes: true}).ID)
	edited = strings.Replace(readFile(t, root, "lib/README.md"), "does lib", "does lib, and my correction", 1)
	writeFile(t, root, "lib/README.md", edited)
	git(t, root, "add", "-A")
	documentedRun(t, root, func(d *stateDoc) {
		d.Purpose = "regenerated " + d.Path
		d.Evidence[0].Claim = d.Purpose
	})
	return root, edited, plan(t, root, Options{InTreeReadmes: true})
}

func TestAHandEditedBlockIsAConflictSurfacedAtTheGate(t *testing.T) {
	_, _, change := handEditedReadme(t)
	conflict := artifactFor(t, change, "lib/README.md")
	if conflict.Action != ActionConflict || !strings.Contains(conflict.Reason, "edited by hand") || conflict.Digest != "" {
		t.Fatalf("%+v", conflict)
	}
	if summary := Summary(change); !strings.Contains(summary, "CONFLICTS") || !strings.Contains(summary, "lib/README.md") {
		t.Errorf("a conflict must be visible at the gate:\n%s", summary)
	}
}

func TestAHandEditedBlockIsNeverOverwrittenAndIsReportedAsWithheld(t *testing.T) {
	root, edited, change := handEditedReadme(t)
	result := approveAndApply(t, root, change.ID)
	if got := readFile(t, root, "lib/README.md"); got != edited {
		t.Errorf("the hand-edited README was overwritten:\n%s", got)
	}
	found := false
	for _, withheld := range result.Withheld {
		found = found || withheld == "lib/README.md"
	}
	if !found {
		t.Errorf("the conflict must be reported as withheld: %+v", result)
	}
}

func TestAnUnchangedGeneratedBlockIsNotRewritten(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{InTreeReadmes: true}).ID)
	change := plan(t, root, Options{InTreeReadmes: true})
	if got := artifactFor(t, change, "lib/README.md").Action; got != ActionUnchanged {
		t.Errorf("re-planning an unchanged tree must stage nothing for it, got %s", got)
	}
}

func TestASidecarThatAlreadyMatchesIsUnchangedToo(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	change := plan(t, root, Options{})
	for _, artifact := range change.Artifacts {
		if artifact.Kind == KindSidecarReadme && artifact.Action != ActionUnchanged {
			t.Errorf("%+v", artifact)
		}
	}
}
