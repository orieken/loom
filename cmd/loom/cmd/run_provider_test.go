package cmd

// Roadmap L3.17: the provider belongs to the run, not to one invocation.
//
// `--provider` was a flag on the invocation and run-state.json did not
// record it, so the resume command the executor itself printed carried no
// provider. Following it moved a free mock run onto the real binary
// mid-flight and billed $1.69 — the tool suggested the command that did it.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// storeWithProvider writes run state recording the provider it started on.
func storeWithProvider(t *testing.T, provider string) *orchestrator.StateStore {
	t.Helper()
	state := orchestrator.NewRunState("deliver-feature", orchestrator.CreatedByExecutor)
	state.Provider = provider
	store := orchestrator.NewStateStore(filepath.Join(t.TempDir(), orchestrator.RunStateFileName))
	if err := store.Save(state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	return store
}

// commandWithProviderFlag mirrors how runCmd declares the flag, so
// Changed() reports what it would during a real invocation.
func commandWithProviderFlag(t *testing.T, passed string) *cobra.Command {
	t.Helper()
	command := &cobra.Command{Use: "run"}
	command.Flags().StringVar(&runArgs.provider, "provider", defaultProvider, "")
	if passed != "" {
		if err := command.Flags().Set("provider", passed); err != nil {
			t.Fatalf("set provider: %v", err)
		}
	}
	return command
}

// The failing path, exactly: resume with no --provider at all. The flag
// default is "claude", so without the recorded value this silently becomes
// a real run.
func TestResumeWithoutTheFlagAdoptsTheRecordedProvider(t *testing.T) {
	command := commandWithProviderFlag(t, "")
	got, err := resolveProvider(command, storeWithProvider(t, "mock"))
	if err != nil {
		t.Fatalf("resolveProvider: %v", err)
	}
	if got != "mock" {
		t.Errorf("resolved %q, want mock — a run resumed without the flag must stay on its own provider", got)
	}
}

// Half a run against each provider is not something anyone asked for, so a
// contradiction is refused rather than silently resolved either way.
func TestAContradictingProviderIsRefusedNamingBoth(t *testing.T) {
	command := commandWithProviderFlag(t, "claude")
	_, err := resolveProvider(command, storeWithProvider(t, "mock"))
	if err == nil {
		t.Fatal("resuming a mock run with --provider claude was allowed")
	}
	for _, want := range []string{"mock", "claude"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
}

// Passing the provider the run already uses is agreement, not conflict.
func TestPassingTheSameProviderIsAccepted(t *testing.T) {
	command := commandWithProviderFlag(t, "mock")
	got, err := resolveProvider(command, storeWithProvider(t, "mock"))
	if err != nil {
		t.Fatalf("resolveProvider rejected a matching provider: %v", err)
	}
	if got != "mock" {
		t.Errorf("resolved %q, want mock", got)
	}
}

// State written before L3.17 records no provider. It must still resume, on
// the flag, rather than failing on a field that did not exist yet.
func TestStateWithoutARecordedProviderFallsBackToTheFlag(t *testing.T) {
	command := commandWithProviderFlag(t, "mock")
	got, err := resolveProvider(command, storeWithProvider(t, ""))
	if err != nil {
		t.Fatalf("resolveProvider: %v", err)
	}
	if got != "mock" {
		t.Errorf("resolved %q, want the flag value for state that records none", got)
	}
}

// The printed resume command has to reproduce the run. Omitting the flag is
// what caused the incident, so a non-default provider is spelled out; the
// default is left off to keep the common case clean.
func TestPrintedResumeCommandNamesANonDefaultProvider(t *testing.T) {
	if got := providerFlagFor("mock"); got != " --provider mock" {
		t.Errorf("providerFlagFor(mock) = %q, want the flag spelled out", got)
	}
	if got := providerFlagFor(defaultProvider); got != "" {
		t.Errorf("providerFlagFor(default) = %q, want empty", got)
	}
	if got := providerFlagFor(""); got != "" {
		t.Errorf("providerFlagFor(unset) = %q, want empty", got)
	}
}
