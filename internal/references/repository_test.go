package references_test

// The fitness function: no live file in this repository points at an agent,
// skill or workflow that does not exist (roadmap L3.61, ADR-009).
//
// When something is removed, add its name to `retired` with the reason; the
// check then lists every place that still names it. Historical records are
// excluded because they describe the past accurately and are not rewritten.

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/references"
)

const repositoryRoot = "../.."

// retired maps each removed agent, skill or workflow name to why it went.
var retired = map[string]string{}

// placeholders are names used only as illustrations in example output.
var placeholders = map[string]bool{
	"foo": true, "bar": true, "alpha": true, "beta": true, "new-agent": true, "old-agent": true, "dev": true,
}

// historical files record the past; each entry says why it is not rewritten.
var historical = []string{
	"*CHANGELOG.md",      // version history names what existed at the time
	"docs/adrs/",         // decisions, including the ones that removed things
	"docs/roadmaps/",     // plans and their shipped history
	"docs/prompts/done/", // executed handoffs
	"docs/aos/prompts/",  // executed AOS phase handoffs
	"docs/blog-posts/",   // published writing
	"docs/features/",     // archived deliveries
	"docs/audits/",       // dated snapshots
	"docs/lessons-learned/",
	"tests/agents/", // golden inputs and outputs, which cite fictional files
	"*_test.go",     // test fixtures build fictional trees
	"internal/references/testdata/",
}

func TestNoLiveFileReferencesSomethingThatDoesNotExist(t *testing.T) {
	findings, err := references.Scan(repositoryRoot, trackedFiles(t), references.Rules{
		Retired: retired, Placeholders: placeholders, Historical: historical,
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("%s:%d — %s %s", finding.File, finding.Line, finding.Reference, finding.Problem)
	}
}

func trackedFiles(t *testing.T) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "ls-files")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	return strings.Fields(string(output))
}
