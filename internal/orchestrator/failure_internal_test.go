package orchestrator

import (
	"context"
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

// A usage or session limit repeats for every call until it resets; a quick
// retry spends an attempt on a certainty.
func TestClassifyFailure(t *testing.T) {
	unavailable := fmt.Errorf("stage %q: %w: session limit", "developer", ErrProviderUnavailable)
	cases := []struct {
		name    string
		failure stageFailure
		want    FailureClass
	}{
		{"a usage limit", stageFailure{err: unavailable, class: FailureTransient}, FailureUnavailable},
		{"a cancellation", stageFailure{err: fmt.Errorf("%w: %w", context.Canceled, ErrProviderUnavailable)}, FailureTransient},
		{"a classified failure", stageFailure{err: errors.New("boom"), class: FailureTransient}, FailureTransient},
		{"an unclassified failure", stageFailure{err: errors.New("boom")}, FailureTerminal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := classifyFailure(testCase.failure); got != testCase.want {
				t.Errorf("class = %q, want %q", got, testCase.want)
			}
		})
	}
}
