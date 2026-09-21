package claude

// Roadmap L3.20, papercuts 1 and 4 — both from the second real run.

import (
	"bytes"
	"strings"
	"testing"
)

// Under `--output-format json` the CLI writes its own errors to STDOUT.
// Reporting only stderr produced `exit status 1 — stderr: ` with nothing
// after it, and the actual cause — a usage limit — was unrecoverable from
// the run record.
func TestFailureDetailReadsTheStreamTheCLIActuallyUsed(t *testing.T) {
	detail := failureDetail(bytes.NewBufferString(`{"error":"usage limit reached"}`), &bytes.Buffer{})
	if !strings.Contains(detail, "usage limit reached") {
		t.Errorf("stdout diagnosis lost: %q", detail)
	}
}

func TestFailureDetailStillReportsStderr(t *testing.T) {
	detail := failureDetail(&bytes.Buffer{}, bytes.NewBufferString("boom"))
	if !strings.Contains(detail, "boom") {
		t.Errorf("stderr lost: %q", detail)
	}
}

// Silence is a finding of its own, and "" would read as a truncated
// message rather than as a process that said nothing.
func TestFailureDetailSaysWhenBothStreamsAreEmpty(t *testing.T) {
	detail := failureDetail(&bytes.Buffer{}, &bytes.Buffer{})
	if !strings.Contains(detail, "wrote nothing") {
		t.Errorf("empty streams should say so, got %q", detail)
	}
}

// An untyped stage writes the model's output verbatim, so a model fencing
// its whole answer leaves the fence in the persisted artifact.
func TestUnfenceArtifact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"wrapped whole", "```markdown\n# Title\n\nBody\n```", "# Title\n\nBody\n"},
		{"wrapped, no language", "```\n# Title\n```", "# Title\n"},
		{"not fenced", "# Title\n\nBody\n", "# Title\n\nBody\n"},
		// A fence INSIDE an artifact is content. The tech writer quoting a
		// code sample is the obvious case, and stripping it would eat the
		// document's first line.
		{"fence is content, not a wrapper", "# Title\n\n```go\nx := 1\n```\n", "# Title\n\n```go\nx := 1\n```\n"},
		{"text after the closing fence means not a wrapper", "```\n# Title\n```\n\nTrailing prose\n", "```\n# Title\n```\n\nTrailing prose\n"},
		{"unterminated fence is left alone", "```markdown\n# Title\n", "```markdown\n# Title\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := unfenceArtifact(testCase.in); got != testCase.want {
				t.Errorf("unfenceArtifact(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}
