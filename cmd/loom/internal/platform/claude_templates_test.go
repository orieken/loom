package platform_test

import (
	"io/fs"
	"testing"

	loom "github.com/orieken/loom"
	"github.com/orieken/loom/cmd/loom/internal/platform"
)

// copyRecorder records what CopyIfMissing was asked to copy, and from where.
type copyRecorder struct {
	recordingWriter
	sources map[string]string
}

func (recorder *copyRecorder) CopyIfMissing(source, destination string) (bool, error) {
	recorder.sources[destination] = source
	return recorder.recordingWriter.CopyIfMissing(source, destination)
}

// The factory's work-order template reaches a project the same way the spec
// template does: copied once, never overwritten (roadmap L3.64).
func TestClaudeInstallCopiesTheIntakeTemplateIfMissing(t *testing.T) {
	rules, err := platform.ParseRuleSet("go")
	if err != nil {
		t.Fatalf("parse rule filter: %v", err)
	}
	recorder := &copyRecorder{sources: map[string]string{}}
	environment := platform.Environment{Content: loom.FrameworkFS, Files: recorder, Rules: rules}
	result, err := platform.Install("claude-code", environment)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if got := recorder.sources["intake"]; got != "templates/claude-feature-team/intake" {
		t.Errorf("intake copied from %q, want the team template's intake directory", got)
	}
	if !contains(result.Paths, "intake") {
		t.Errorf("installed paths %v do not report intake", result.Paths)
	}
}

// The copy is only as good as what the binary embeds.
func TestTheBinaryEmbedsTheIntakeTemplate(t *testing.T) {
	for _, name := range []string{"TEMPLATE.md", "README.md"} {
		if _, err := fs.Stat(loom.FrameworkFS, "templates/claude-feature-team/intake/"+name); err != nil {
			t.Errorf("intake/%s is not embedded: %v", name, err)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
