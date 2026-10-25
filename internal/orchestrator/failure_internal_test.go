package orchestrator

import (
	"errors"
	"fmt"
	"testing"
)

func TestArtifactFailureClass(t *testing.T) {
	invalid := fmt.Errorf("stage %q returned %w: unknown field", "analyst", ErrInvalidState)
	cases := []struct {
		name  string
		stage Stage
		err   error
		want  FailureClass
	}{
		{"a provider stage's schema violation", Stage{ID: "analyst"}, invalid, FailureTransient},
		{"an internal stage's schema violation", Stage{ID: "router", Internal: true}, invalid, FailureTerminal},
		{"any other artifact failure", Stage{ID: "developer"}, errors.New("claimed a file it did not write"), FailureTerminal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := artifactFailureClass(testCase.stage, testCase.err); got != testCase.want {
				t.Errorf("class = %q, want %q", got, testCase.want)
			}
		})
	}
}
