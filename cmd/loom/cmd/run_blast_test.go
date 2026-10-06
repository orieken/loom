package cmd

import (
	"testing"

	"github.com/orieken/loom/internal/repoquery/fixture"
)

func blastFixture(t *testing.T) string {
	t.Helper()
	return fixture.Indexed(t, map[string]string{
		"go.mod":         "module example.com/x\n\ngo 1.22\n",
		"svc/svc.go":     "package svc\n\nimport \"example.com/x/store\"\n\nfunc Run() { store.Get() }\n",
		"store/store.go": "package store\n\nfunc Get() {}\n",
	})
}

func TestTheBlastRadiusResolverCarriesTheGraphRevisionAndFreshness(t *testing.T) {
	resolution, err := blastRadiusResolverFor(blastFixture(t))([]string{"store"}, 0)
	if err != nil || !resolution.Fresh || len(resolution.GraphSHA) != 40 {
		t.Errorf("%+v %v", resolution, err)
	}
}

func TestTheBlastRadiusResolverAnswersFromTheRepoGraph(t *testing.T) {
	resolution, err := blastRadiusResolverFor(blastFixture(t))([]string{"store"}, 0)
	if err != nil || len(resolution.Impacted) != 1 {
		t.Fatalf("%+v %v", resolution, err)
	}
	if got := resolution.Impacted[0]; got.Path != "svc" || got.Distance != 1 || got.Confidence != "exact" {
		t.Errorf("%+v", got)
	}
}

func TestTheBlastRadiusResolverReportsAMissingGraphAndAnUnknownPath(t *testing.T) {
	if _, err := blastRadiusResolverFor(t.TempDir())([]string{"x"}, 1); err == nil {
		t.Error("a repo that was never indexed has no radius to compute")
	}
	root := fixture.Indexed(t, map[string]string{"go.mod": "module example.com/x\n\ngo 1.22\n", "a/a.go": "package a\n"})
	if _, err := blastRadiusResolverFor(root)([]string{"typo"}, 1); err == nil {
		t.Error("an unknown seed must be an error, never a smaller radius")
	}
}
