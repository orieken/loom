package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// The markdown pipeline (the deliver-feature skill) routes with a model but
// must not compute its own integrity hashes — that is the defect roadmap
// L2.12 closes. `loom state` is how it records checkpoints: Go reads the
// artifact and hashes it. No subcommand accepts a caller-supplied digest.

type stateFlags struct {
	spec     string
	stage    string
	artifact string
	gate     string
	asJSON   bool
}

var stateArgs stateFlags

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Record and verify markdown-pipeline checkpoints (executor-owned state)",
	Long: `Record and verify delivery checkpoints in run-state.json.

The deliver-feature skill calls these subcommands so that artifact digests
are computed by this binary rather than by a model (roadmap L2.12). Runs
executed by 'loom run' own their state directly and need none of this.

'state approve' records a human's gate approval for audit. It does NOT
enforce the gate: enforcement exists only for stages run by 'loom run'.`,
	Args: cobra.NoArgs,
}

var stateRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a completed stage and hash its artifact",
	Args:  cobra.NoArgs,
	RunE:  runStateRecord,
}

var stateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Re-hash every recorded artifact and report mismatches",
	Args:  cobra.NoArgs,
	RunE:  runStateVerify,
}

var stateApproveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Record a gate approval (audit record, not enforcement)",
	Args:  cobra.NoArgs,
	RunE:  runStateApprove,
}

var stateTimelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Print the recorded event timeline for a run",
	Args:  cobra.NoArgs,
	RunE:  runStateTimeline,
}

var stateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show recorded stages, approvals, and where the run stands",
	Args:  cobra.NoArgs,
	RunE:  runStateShow,
}

func init() {
	rootCmd.AddCommand(stateCmd)
	stateCmd.AddCommand(stateRecordCmd, stateVerifyCmd, stateApproveCmd, stateShowCmd, stateTimelineCmd)
	for _, sub := range []*cobra.Command{stateRecordCmd, stateVerifyCmd, stateApproveCmd, stateShowCmd, stateTimelineCmd} {
		sub.Flags().StringVar(&stateArgs.spec, "spec", "", "feature spec markdown file (required)")
		_ = sub.MarkFlagRequired("spec")
	}
	stateRecordCmd.Flags().StringVar(&stateArgs.stage, "stage", "", "stage/agent ID that completed (required)")
	stateRecordCmd.Flags().StringVar(&stateArgs.artifact, "artifact", "", "artifact file the stage produced")
	_ = stateRecordCmd.MarkFlagRequired("stage")
	stateApproveCmd.Flags().StringVar(&stateArgs.gate, "gate", "", "gate name being approved (required)")
	_ = stateApproveCmd.MarkFlagRequired("gate")
	stateShowCmd.Flags().BoolVar(&stateArgs.asJSON, "json", false, "print machine-readable JSON")
	stateTimelineCmd.Flags().BoolVar(&stateArgs.asJSON, "json", false, "print machine-readable JSON")
}

// openTimeline returns the event log beside a spec's run state. Both
// pipelines append to the same file, in the same shape.
func openTimeline(specPath string) (*orchestrator.Timeline, error) {
	workspace, _, err := prepareRunWorkspace(specPath)
	if err != nil {
		return nil, err
	}
	return orchestrator.NewTimeline(filepath.Join(workspace, orchestrator.RunStateFileName)), nil
}

// openMarkdownState loads the state for a spec, creating it on first use.
// It refuses state written by the executor: the two pipelines route
// differently and must not resume each other's runs.
func openMarkdownState(specPath string) (*orchestrator.StateStore, *orchestrator.RunState, error) {
	workspace, input, err := prepareRunWorkspace(specPath)
	if err != nil {
		return nil, nil, err
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	state, err := store.Load()
	if err != nil {
		return nil, nil, err
	}
	if state == nil {
		return store, newMarkdownState(input), nil
	}
	if err := state.CheckCreatedBy(orchestrator.CreatedByMarkdown); err != nil {
		return nil, nil, err
	}
	return store, state, nil
}

func newMarkdownState(input orchestrator.StageInput) *orchestrator.RunState {
	state := orchestrator.NewRunState(orchestrator.DefaultDeliverFeaturePlanName, orchestrator.CreatedByMarkdown)
	state.SpecPath = input.SpecPath
	state.SpecSHA256 = orchestrator.SpecDigest(input.SpecPath)
	state.FeatureName = orchestrator.FeatureNameFromSpec(input.SpecPath)
	return state
}

// requireExistingState is for the subcommands that read a run rather than
// start one — verifying or showing a run nobody recorded is an error, not
// an empty report.
func requireExistingState(specPath string) (*orchestrator.StateStore, *orchestrator.RunState, error) {
	store, state, err := openMarkdownState(specPath)
	if err != nil {
		return nil, nil, err
	}
	if len(state.Stages) == 0 && len(state.Approvals) == 0 {
		return nil, nil, fmt.Errorf("no checkpoints recorded for %s yet", specPath)
	}
	return store, state, nil
}
