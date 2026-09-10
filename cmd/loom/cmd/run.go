package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/planfile"
	"github.com/orieken/loom/internal/policy"
	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/internal/worktree"
	"github.com/spf13/cobra"
)

type runFlags struct {
	spec           string
	resume         bool
	provider       string
	plan           string
	approve        string
	mockHangStage  string
	otelFile       string
	noTelemetry    bool
	dryRunPolicies bool
}

var runArgs runFlags

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the delivery pipeline for a feature spec (experimental)",
	Long: `Execute the delivery pipeline for a feature spec with durable state.

EXPERIMENTAL SKELETON (roadmap M0.4, ADR-006): runs the built-in linear
deliver-feature plan stage by stage via the claude CLI, persisting
run-state.json after every transition. First Ctrl-C checkpoints and exits
cleanly (resume with --resume); a second Ctrl-C kills immediately.

Approval gates (roadmap L2.13) are process interrupts: the executor refuses
to start a gated stage until a human approves its gate. On a terminal you
are asked at the barrier; otherwise the run halts with exit code 3 and
prints the resume command (loom run --spec X --resume --approve <gate>).
Nothing an agent returns can approve a gate.

Not yet implemented (see BUILD-ROADMAP.md): reset-on-edit for approvals
(L2.14), retries/backoff, parallelism (L3.3), policy evaluation (L2.16),
conditional stage routing (L3.1).

Traces (roadmap L3.8) are written as OTLP/JSON to <workspace>/traces.jsonl on
every run; set OTEL_EXPORTER_OTLP_ENDPOINT to also export over OTLP/HTTP, or
--no-telemetry to record nothing.`,
	Args: cobra.NoArgs,
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runArgs.spec, "spec", "", "feature spec markdown file (required)")
	runCmd.Flags().BoolVar(&runArgs.resume, "resume", false, "continue an interrupted run from its checkpoint")
	runCmd.Flags().StringVar(&runArgs.approve, "approve", "", "approve the gate the run is waiting on (requires --resume)")
	runCmd.Flags().StringVar(&runArgs.provider, "provider", "claude", "stage provider: claude or mock")
	runCmd.Flags().StringVar(&runArgs.plan, "plan", orchestrator.DefaultDeliverFeaturePlanName, "pipeline plan to execute")
	runCmd.Flags().StringVar(&runArgs.mockHangStage, "mock-hang-stage", "", "mock provider only: stage ID that hangs until interrupted (testing)")
	_ = runCmd.Flags().MarkHidden("mock-hang-stage")
	runCmd.Flags().StringVar(&runArgs.otelFile, "otel-file", "",
		"write OTLP/JSON traces here (default: <workspace>/"+telemetry.TracesFileName+")")
	runCmd.Flags().BoolVar(&runArgs.noTelemetry, "no-telemetry", false,
		"disable tracing entirely, including the local trace file")
	runCmd.Flags().BoolVar(&runArgs.dryRunPolicies, "dry-run-policies", false,
		"evaluate policies against an existing run's state, print the decisions, and exit without running anything")
	_ = runCmd.MarkFlagRequired("spec")
}

func runRun(cmd *cobra.Command, _ []string) error {
	plan, err := selectPlan(runArgs.plan)
	if err != nil {
		return err
	}
	workspace, input, err := prepareRunWorkspace(runArgs.spec)
	if err != nil {
		return err
	}
	store := orchestrator.NewStateStore(filepath.Join(workspace, orchestrator.RunStateFileName))
	policies, err := loadPolicies(cmd, policy.DefaultDir)
	if err != nil {
		return err
	}
	// Dry-run reads a finished run, so it must not go through the resume
	// check — that check refuses to start over existing state, which is
	// exactly the state dry-run exists to evaluate against.
	if runArgs.dryRunPolicies {
		return runDryRunPolicies(cmd, store, policies)
	}
	if err := checkResumeState(store, runArgs.resume); err != nil {
		return err
	}
	provider, err := selectProvider(plan, runArgs.provider, runArgs.mockHangStage)
	if err != nil {
		return err
	}
	return executeRun(cmd, runSetup{plan: plan, provider: provider, store: store, input: input, policies: policies})
}

// runSetup is what a run needs to start, gathered rather than passed as six
// positional arguments.
type runSetup struct {
	plan     orchestrator.Plan
	provider orchestrator.Provider
	store    *orchestrator.StateStore
	input    orchestrator.StageInput
	policies []policy.Policy
}

func executeRun(cmd *cobra.Command, setup runSetup) error {
	plan, provider, store, input := setup.plan, setup.provider, setup.store, setup.input
	executor := orchestrator.NewExecutor(provider, store)
	// Fingerprint the repository around each stage so one that edits source
	// it never declared it would edit is noticed (roadmap L3.30).
	executor.WithWorkTree(worktree.New(input.ProjectRoot))
	executor.OnPostureError(reportPostureErrorOnce(cmd))
	// Reproduce a stage's claim that the suite passes, rather than
	// believing it (roadmap L2.24).
	executor.WithMeasurementVerifier(newTestVerifier(cmd, input.ProjectRoot))
	executor.OnClaimWarning(reportClaimWarning(cmd))
	executor.WithPolicies(setup.policies)
	executor.OnPolicyDecision(func(decision policy.Decision) { reportPolicyDecision(cmd, decision) })
	stopTelemetry, err := startTelemetry(cmd, executor, input.WorkspaceDir)
	if err != nil {
		return err
	}
	defer stopTelemetry()
	executor.OnStale(func(stale []orchestrator.StaleStage) { reportStaleStages(cmd, stale) })
	executor.OnApprovalReset(func(reset *orchestrator.StaleApprovalError) { reportApprovalReset(cmd, reset) })
	executor.OnBaselineError(func(err error) { cmd.PrintErrf("warning: %v\n", err) })
	executor.OnRoute(func(summary orchestrator.RouteSummary) { reportRoute(cmd, summary, input.WorkspaceDir) })
	executor.OnLoopRound(func(round orchestrator.LoopRound) { reportLoopRound(cmd, round) })
	if err := applyApproveFlag(executor, runArgs.approve, runArgs.resume); err != nil {
		return err
	}
	ctx, stopSignals := interruptibleContext(cmd)
	defer stopSignals()
	// Record the episode however the run ends — completed, halted at a gate,
	// interrupted, or failed. A store that only knew about clean completions
	// would miss the runs anyone actually investigates (roadmap L3.5).
	defer recordEpisode(cmd, input.WorkspaceDir, orchestrator.FeatureNameFromSpec(input.SpecPath))
	cmd.Printf("Running plan %q (%d stages) — state: %s\n", plan.Name, len(plan.Stages), store.Path())
	err = runWithGates(ctx, cmd, executor, plan, input)
	if errors.Is(err, context.Canceled) {
		cmd.Printf("Interrupted — checkpoint saved. Continue with: loom run --spec %s --resume\n", runArgs.spec)
		return err
	}
	if err != nil {
		return err
	}
	cmd.Printf("Plan %q completed. Artifacts: %s\n", plan.Name, input.WorkspaceDir)
	reportRunUsage(cmd, store)
	return nil
}

// interruptibleContext cancels on the first SIGINT/SIGTERM so the executor
// persists a clean checkpoint; a second signal kills the process immediately.
func interruptibleContext(cmd *cobra.Command) (context.Context, func()) {
	ctx, cancel := context.WithCancel(cmd.Context())
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		cmd.PrintErrln("interrupt received — checkpointing (press Ctrl-C again to kill)")
		cancel()
		<-signals
		os.Exit(130)
	}()
	return ctx, func() { signal.Stop(signals); cancel() }
}

// selectPlan resolves --plan to the built-in plan or to a plan file
// (roadmap L3.27). The built-in stays in Go and remains the default: it is
// the one pipeline every run has exercised, and putting it behind the loader
// on day one would make a malformed embed break every run rather than only
// the custom ones. A test asserts it round-trips through the format
// unchanged, which is what makes the format sufficient rather than merely
// present.
func selectPlan(name string) (orchestrator.Plan, error) {
	if name == orchestrator.DefaultDeliverFeaturePlanName {
		return orchestrator.DefaultDeliverFeaturePlan(), nil
	}
	path, err := findPlanFile(name)
	if err != nil {
		return orchestrator.Plan{}, err
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return orchestrator.Plan{}, fmt.Errorf("read plan %q: %w", name, err)
	}
	return planfile.Parse(source, path)
}

// planSearchPath is where a plan file may live, project-local first: a
// project's own plan overrides one the framework installed under the same
// name, which is the same precedence every other .claude/ resource has.
func planSearchPath(name string) []string {
	return []string{
		filepath.Join(".claude", "plans", name+".yaml"),
		filepath.Join("shared", "plans", name+".yaml"),
	}
}

func findPlanFile(name string) (string, error) {
	candidates := planSearchPath(name)
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown plan %q — %q is built in, and no plan file was found at %s",
		name, orchestrator.DefaultDeliverFeaturePlanName, strings.Join(candidates, " or "))
}

// prepareRunWorkspace validates the spec and creates the feature workspace
// (.claude/feature-workspace/<feature>/, same location the markdown pipeline
// uses — see deliver-feature SKILL.md "Workspace Path Resolution").
func prepareRunWorkspace(specPath string) (string, orchestrator.StageInput, error) {
	absSpec, err := filepath.Abs(specPath)
	if err != nil {
		return "", orchestrator.StageInput{}, fmt.Errorf("resolve spec path: %w", err)
	}
	if _, err := os.Stat(absSpec); err != nil {
		return "", orchestrator.StageInput{}, fmt.Errorf("feature spec not found: %w", err)
	}
	feature := orchestrator.FeatureNameFromSpec(absSpec)
	workspace, err := filepath.Abs(filepath.Join(".claude", "feature-workspace", feature))
	if err != nil {
		return "", orchestrator.StageInput{}, fmt.Errorf("resolve workspace path: %w", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", orchestrator.StageInput{}, fmt.Errorf("create workspace: %w", err)
	}
	// The workspace is created under the current directory, so that is the
	// project a manifest's repo-relative paths resolve against (L3.25).
	root, err := filepath.Abs(".")
	if err != nil {
		return "", orchestrator.StageInput{}, fmt.Errorf("resolve project root: %w", err)
	}
	return workspace, orchestrator.StageInput{SpecPath: absSpec, WorkspaceDir: workspace, ProjectRoot: root}, nil
}

// checkResumeState enforces the resume contract: --resume requires existing
// state, and a fresh run refuses to start over existing state.
func checkResumeState(store *orchestrator.StateStore, resume bool) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	if resume && state == nil {
		return fmt.Errorf("--resume given but no run state exists at %s — start without --resume", store.Path())
	}
	if !resume && state != nil {
		return fmt.Errorf("run state already exists at %s — continue with --resume, or delete the file to start over", store.Path())
	}
	return nil
}
